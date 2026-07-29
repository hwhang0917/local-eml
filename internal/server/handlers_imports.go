package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

const (
	maxUploadSize     = 1 << 30
	multipartMemBytes = 32 << 20
)

func newImportID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(multipartMemBytes); err != nil {
		http.Error(w, "parse multipart: "+err.Error(), http.StatusBadRequest)
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["file"]) == 0 {
		http.Error(w, "missing 'file' field", http.StatusBadRequest)
		return
	}
	files := r.MultipartForm.File["file"]

	paths, names, err := saveUploads(files)
	if err != nil {
		removeAll(paths)
		http.Error(w, "save uploads: "+err.Error(), http.StatusInternalServerError)
		return
	}

	kind := detectKind(files)
	importID := newImportID()
	sourceName := files[0].Filename
	if len(files) > 1 {
		sourceName = fmt.Sprintf("(%d files)", len(files))
	}

	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: kind,
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		removeAll(paths)
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var src importer.Source
	switch kind {
	case "zip":
		src = importer.NewZipSource(paths[0])
	case "mbox":
		src = importer.NewMboxSource(paths[0])
	case "file":
		src = importer.NewLocalSource(sourceName, paths, names, false)
	default: // "dir"
		src = importer.NewLocalSource(sourceName, paths, names, true)
	}

	slog.Info("import requested",
		slog.String("import_id", importID),
		slog.String("kind", kind),
		slog.String("source", sourceName),
		slog.Int("files", len(files)),
	)
	go s.runJob(importID, kind, src, func() { removeAll(paths) })

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      kind,
	})
}

func (s *Server) runJob(importID, kind string, src importer.Source, cleanup func(), afterDone ...func()) {
	defer cleanup()
	ctx, release := s.Canceller.Register(context.Background(), importID)
	defer release()
	_ = s.Store.UpdateImportStatus(ctx, importID, "running", false)

	log := slog.Default().With(
		slog.String("import_id", importID),
		slog.String("kind", kind),
	)

	job := &importer.Job{
		Importer: s.Importer,
		Hub:      s.Hub,
		Store:    s.Store,
		ID:       importID,
		Logger:   log,
	}

	defer func() {
		if rec := recover(); rec != nil {
			log.Error("import job panic", slog.Any("panic", rec))
			_ = s.Store.UpdateImportStatus(context.Background(), importID, "error", true)
			s.Hub.Publish(importID, importer.Event{Type: "error", Message: fmt.Sprint(rec)})
			s.Hub.Close(importID)
		}
	}()

	job.RunSource(ctx, src)

	for _, fn := range afterDone {
		if fn != nil {
			fn()
		}
	}
}

// handleResync re-imports every .eml already in the blob directory. Rows the
// database knows are skipped as duplicates by their hash; files without a row
// get parsed and indexed. Running it as a normal import job means progress,
// cancellation and the job list all come for free.
func (s *Server) handleResync(w http.ResponseWriter, r *http.Request) {
	dir := s.Importer.Paths.EML
	entries, err := os.ReadDir(dir)
	if err != nil {
		http.Error(w, "read blob dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var paths, names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".eml") {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
		names = append(names, e.Name())
	}

	importID := newImportID()
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: "resync",
		SourceName: dir,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	src := importer.NewLocalSource("library resync", paths, names, true)

	slog.Info("resync requested",
		slog.String("import_id", importID),
		slog.Int("files", len(paths)),
	)
	// Cleanup must be a no-op: unlike uploads, these paths are the blobs.
	go s.runJob(importID, "resync", src, func() {})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      "resync",
		"total":     len(paths),
	})
}

func (s *Server) handleImportStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	imp, err := s.Store.GetImport(r.Context(), id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, imp)
}

func (s *Server) handleImportEvents(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	ch, cancel := s.Hub.Subscribe(id)
	defer cancel()

	if imp, err := s.Store.GetImport(r.Context(), id); err == nil &&
		(imp.Status == "done" || imp.Status == "error") {
		ev := importer.Event{
			Type: "done", Phase: "Completed",
			Processed: imp.Processed, Total: imp.Total,
		}
		if imp.Status == "error" {
			ev.Type = "error"
			ev.Phase = "Failed"
		}
		writeSSE(w, flusher, ev)
		return
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, flusher, ev)
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, ev importer.Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func detectKind(files []*multipart.FileHeader) string {
	switch {
	case len(files) == 1 && strings.HasSuffix(strings.ToLower(files[0].Filename), ".zip"):
		return "zip"
	case len(files) == 1 && strings.HasSuffix(strings.ToLower(files[0].Filename), ".mbox"):
		return "mbox"
	case len(files) == 1:
		return "file"
	default:
		return "dir"
	}
}

func saveUploads(files []*multipart.FileHeader) ([]string, []string, error) {
	paths := make([]string, 0, len(files))
	names := make([]string, 0, len(files))
	for _, h := range files {
		path, err := saveOne(h)
		if err != nil {
			return paths, names, err
		}
		paths = append(paths, path)
		names = append(names, h.Filename)
	}
	return paths, names, nil
}

func saveOne(h *multipart.FileHeader) (string, error) {
	src, err := h.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, src); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func removeAll(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

func (s *Server) handleImportS3(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccessKeyID     string `json:"accessKeyId"`
		SecretAccessKey string `json:"secretAccessKey"`
		SessionToken    string `json:"sessionToken"`
		Region          string `json:"region"`
		Bucket          string `json:"bucket"`
		Prefix          string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	body.Bucket = strings.TrimSpace(body.Bucket)
	body.Prefix = strings.TrimSpace(body.Prefix)
	if body.Bucket == "" {
		http.Error(w, "bucket is required", http.StatusBadRequest)
		return
	}

	importID := newImportID()
	sourceName := fmt.Sprintf("s3://%s/%s", body.Bucket, body.Prefix)
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: "s3",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	src := importer.NewS3Source(importer.S3Config{
		AccessKeyID:     body.AccessKeyID,
		SecretAccessKey: body.SecretAccessKey,
		SessionToken:    body.SessionToken,
		Region:          body.Region,
		Bucket:          body.Bucket,
		Prefix:          body.Prefix,
	})

	slog.Info("import requested",
		slog.String("import_id", importID),
		slog.String("kind", "s3"),
		slog.String("bucket", body.Bucket),
		slog.String("prefix", body.Prefix),
		slog.String("region", body.Region),
		slog.Bool("static_creds", body.AccessKeyID != ""),
	)
	go s.runJob(importID, "s3", src, func() {})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      "s3",
	})
}

func (s *Server) handleImportImap(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ProfileID int64  `json:"profile_id,omitempty"`
		Host      string `json:"host"`
		Port      int    `json:"port"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		Folder    string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}

	cfg, profile, err := s.resolveIMAPConfig(r.Context(), body.ProfileID, importer.IMAPConfig{
		Host:     strings.TrimSpace(body.Host),
		Port:     body.Port,
		Username: strings.TrimSpace(body.Username),
		Password: body.Password,
		Folder:   strings.TrimSpace(body.Folder),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	importID := newImportID()
	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	sourceName := fmt.Sprintf("imap://%s@%s/%s", cfg.Username, cfg.Host, folder)
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: "imap",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// If the user supplied a fresh password while sync is enabled on a saved
	// profile, lock it in so the background poller can use it later.
	if profile != nil && profile.SyncEnabled && body.Password != "" && s.Secret != nil {
		if blob, encErr := s.Secret.Encrypt([]byte(body.Password)); encErr == nil {
			if err := s.Store.SetIMAPProfilePassword(r.Context(), profile.ID, blob); err != nil {
				slog.Warn("store imap password failed",
					slog.Int64("profile_id", profile.ID), slog.String("err", err.Error()))
			}
		}
	}

	src := importer.NewIMAPSource(cfg)

	slog.Info("import requested",
		slog.String("import_id", importID),
		slog.String("kind", "imap"),
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.String("username", cfg.Username),
		slog.String("folder", folder),
		slog.Bool("incremental", cfg.Incremental),
	)

	var afterDone func()
	if profile != nil && profile.SyncEnabled {
		profileID := profile.ID
		afterDone = func() { s.persistIMAPSyncState(profileID, src) }
	}
	go s.runJob(importID, "imap", src, func() {}, afterDone)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      "imap",
	})
}

// resolveIMAPConfig fills in defaults, optionally looks up a saved profile,
// uses the stored encrypted password as a fallback when sync is enabled, and
// applies incremental sync state. It returns a validation error if the final
// config doesn't have everything needed to log in.
func (s *Server) resolveIMAPConfig(ctx context.Context, profileID int64, base importer.IMAPConfig) (importer.IMAPConfig, *store.IMAPProfile, error) {
	cfg := base
	var profile *store.IMAPProfile
	if profileID > 0 {
		p, err := s.Store.GetIMAPProfile(ctx, profileID)
		if err != nil {
			return cfg, nil, fmt.Errorf("load profile: %w", err)
		}
		profile = p
		if cfg.Host == "" {
			cfg.Host = p.Host
		}
		if cfg.Port == 0 && p.Port != nil {
			cfg.Port = *p.Port
		}
		if cfg.Username == "" {
			cfg.Username = p.Username
		}
		if cfg.Folder == "" && p.Folder != nil {
			cfg.Folder = *p.Folder
		}
		if cfg.Password == "" && p.SyncEnabled && p.HasPassword && s.Secret != nil {
			blob, err := s.Store.GetIMAPProfilePassword(ctx, p.ID)
			if err == nil && len(blob) > 0 {
				plain, derr := s.Secret.Decrypt(blob)
				if derr == nil {
					cfg.Password = string(plain)
				}
			}
		}
		if p.SyncEnabled && p.UIDValidity != nil && p.LastUID != nil {
			cfg.Incremental = true
			cfg.ExpectedUIDValidity = *p.UIDValidity
			cfg.SinceUID = *p.LastUID
		} else if p.SyncEnabled {
			// First sync — Incremental stays false so we fetch everything once
			// and then start tracking. SyncReporter still records the result.
		}
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		return cfg, profile, fmt.Errorf("host, username and password are required")
	}
	return cfg, profile, nil
}

// persistIMAPSyncState records the new UIDVALIDITY + max UID on the profile so
// the next sync picks up where this one left off.
func (s *Server) persistIMAPSyncState(profileID int64, src importer.Source) {
	reporter, ok := src.(importer.SyncReporter)
	if !ok {
		return
	}
	res, ok := reporter.SyncResult()
	if !ok {
		return
	}
	if err := s.Store.UpdateIMAPProfileSyncState(
		context.Background(), profileID,
		res.UIDValidity, res.MaxUID, time.Now().Unix(),
	); err != nil {
		slog.Warn("persist imap sync state failed",
			slog.Int64("profile_id", profileID), slog.String("err", err.Error()))
	}
}
