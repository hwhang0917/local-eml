package server

import (
	"context"
	"encoding/json"
	"fmt"
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

func (s *Server) handleImportUpload(w http.ResponseWriter, r *http.Request) {
	files, ok := uploadedFiles(w, r)
	if !ok {
		return
	}
	paths, names, err := saveUploads(files)
	if err != nil {
		removeAll(paths)
		http.Error(w, "save uploads: "+err.Error(), http.StatusInternalServerError)
		return
	}

	kind := detectKind(files)
	sourceName := files[0].Filename
	if len(files) > 1 {
		sourceName = fmt.Sprintf("(%d files)", len(files))
	}
	id, err := s.queueJob(r.Context(), kind, sourceName)
	if err != nil {
		removeAll(paths)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var src importer.Source
	switch kind {
	case kindZip:
		src = importer.NewZipSource(paths[0])
	case kindMbox:
		src = importer.NewMboxSource(paths[0])
	case kindPst:
		src = importer.NewPstSource(paths[0])
	case kindFile:
		src = importer.NewLocalSource(sourceName, paths, names)
	default: // kindDir
		src = importer.NewLocalEMLSource(sourceName, paths, names)
	}

	slog.Info("import requested",
		slog.String("import_id", id),
		slog.String("kind", kind),
		slog.String("source", sourceName),
		slog.Int("files", len(files)),
	)
	go s.runJob(id, kind, src, func() { removeAll(paths) }, nil)
	writeJobAccepted(w, "import_id", id, kind, nil)
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
		if e.IsDir() || !importer.IsEML(e.Name()) {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
		names = append(names, e.Name())
	}

	id, err := s.queueJob(r.Context(), kindResync, dir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	src := importer.NewLocalEMLSource("library resync", paths, names)

	slog.Info("resync requested",
		slog.String("import_id", id),
		slog.Int("files", len(paths)),
	)
	// No cleanup: unlike uploads, these paths are the blobs themselves.
	go s.runJob(id, kindResync, src, nil, nil)
	writeJobAccepted(w, "import_id", id, kindResync, map[string]any{"total": len(paths)})
}

func (s *Server) handleImportStatus(w http.ResponseWriter, r *http.Request) {
	imp, err := s.Store.GetImport(r.Context(), chi.URLParam(r, "id"))
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

	// A job that already ended has no stream left; replay its outcome once.
	if imp, err := s.Store.GetImport(r.Context(), id); err == nil &&
		(imp.Status == store.ImportDone || imp.Status == store.ImportError) {
		ev := importer.Event{
			Type: importer.EventDone, Phase: "Completed",
			Processed: imp.Processed, Total: imp.Total,
		}
		if imp.Status == store.ImportError {
			ev.Type = importer.EventError
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

// detectKind picks the source from the upload: one archive is opened as such,
// one other file is imported as-is, several files are treated as a folder.
func detectKind(files []*multipart.FileHeader) string {
	if len(files) != 1 {
		return kindDir
	}
	switch strings.ToLower(filepath.Ext(files[0].Filename)) {
	case ".zip":
		return kindZip
	case ".mbox":
		return kindMbox
	case ".pst":
		return kindPst
	}
	return kindFile
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
	return copyToTemp(src, "upload-*")
}

func removeAll(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

// s3Body is the JSON both S3 endpoints take.
type s3Body struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	SessionToken    string `json:"sessionToken"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	Prefix          string `json:"prefix"`
}

// decodeS3Config reads and validates an S3 request. On failure the response
// is already written.
func decodeS3Config(w http.ResponseWriter, r *http.Request) (importer.S3Config, bool) {
	var body s3Body
	if !decodeJSON(w, r, &body) {
		return importer.S3Config{}, false
	}
	cfg := importer.S3Config{
		AccessKeyID:     body.AccessKeyID,
		SecretAccessKey: body.SecretAccessKey,
		SessionToken:    body.SessionToken,
		Region:          body.Region,
		Bucket:          strings.TrimSpace(body.Bucket),
		Prefix:          strings.TrimSpace(body.Prefix),
	}
	if cfg.Bucket == "" {
		http.Error(w, "bucket is required", http.StatusBadRequest)
		return importer.S3Config{}, false
	}
	return cfg, true
}

// s3Logger tags log lines with the destination, never the secret.
func s3Logger(cfg importer.S3Config) *slog.Logger {
	return slog.With(
		slog.String("bucket", cfg.Bucket),
		slog.String("prefix", cfg.Prefix),
		slog.String("region", cfg.Region),
		slog.Bool("static_creds", cfg.AccessKeyID != ""),
	)
}

func (s *Server) handleImportS3(w http.ResponseWriter, r *http.Request) {
	cfg, ok := decodeS3Config(w, r)
	if !ok {
		return
	}
	src := importer.NewS3Source(cfg)
	id, err := s.queueJob(r.Context(), kindS3, src.Label())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s3Logger(cfg).Info("import requested", slog.String("import_id", id), slog.String("kind", kindS3))
	go s.runJob(id, kindS3, src, nil, nil)
	writeJobAccepted(w, "import_id", id, kindS3, nil)
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
	if !decodeJSON(w, r, &body) {
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

	src := importer.NewIMAPSource(cfg)
	id, err := s.queueJob(r.Context(), kindIMAP, src.Label())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	syncing := profile != nil && profile.SyncEnabled
	if syncing && body.Password != "" {
		s.rememberIMAPPassword(r.Context(), profile.ID, body.Password)
	}

	slog.Info("import requested",
		slog.String("import_id", id),
		slog.String("kind", kindIMAP),
		slog.String("host", cfg.Host),
		slog.Int("port", cfg.Port),
		slog.String("username", cfg.Username),
		slog.String("folder", cfg.Mailbox()),
		slog.Bool("incremental", cfg.Incremental),
	)

	var afterDone func()
	if syncing {
		profileID := profile.ID
		afterDone = func() { s.persistIMAPSyncState(profileID, src) }
	}
	go s.runJob(id, kindIMAP, src, nil, afterDone)
	writeJobAccepted(w, "import_id", id, kindIMAP, nil)
}

// rememberIMAPPassword stores a freshly supplied password on a sync-enabled
// profile so the background poller can log in later.
func (s *Server) rememberIMAPPassword(ctx context.Context, profileID int64, password string) {
	if s.Secret == nil {
		return
	}
	blob, err := s.Secret.Encrypt([]byte(password))
	if err == nil {
		err = s.Store.SetIMAPProfilePassword(ctx, profileID, blob)
	}
	if err != nil {
		slog.Warn("store imap password failed",
			slog.Int64("profile_id", profileID), slog.String("err", err.Error()))
	}
}

// resolveIMAPConfig fills in what the request left blank from the saved
// profile, falls back to its stored password when sync is enabled, and turns
// on incremental fetching once a cursor exists. It returns a validation error
// if the final config doesn't have everything needed to log in.
func (s *Server) resolveIMAPConfig(ctx context.Context, profileID int64, cfg importer.IMAPConfig) (importer.IMAPConfig, *store.IMAPProfile, error) {
	var profile *store.IMAPProfile
	if profileID > 0 {
		p, err := s.Store.GetIMAPProfile(ctx, profileID)
		if err != nil {
			return cfg, nil, fmt.Errorf("load profile: %w", err)
		}
		profile = p
		cfg = profileDefaults(cfg, p)
		// The stored password only ever goes to the server it was saved for.
		// A request that names another host, port or user must bring its own,
		// or anyone who can reach the API could point the profile at a server
		// they control and collect the secret.
		if cfg.Password == "" && p.SyncEnabled && p.HasPassword && sameIMAPServer(cfg, p) {
			cfg.Password = s.storedIMAPPassword(ctx, p.ID)
		}
		// A first sync fetches everything once; the cursor only exists after.
		if p.SyncEnabled {
			cfg = withSyncCursor(cfg, p)
		}
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		return cfg, profile, fmt.Errorf("host, username and password are required")
	}
	return cfg, profile, nil
}

// sameIMAPServer reports whether cfg logs in to the profile's own server.
func sameIMAPServer(cfg importer.IMAPConfig, p *store.IMAPProfile) bool {
	port := 0
	if p.Port != nil {
		port = *p.Port
	}
	return cfg.Host == p.Host && cfg.Port == port && cfg.Username == p.Username
}

// storedIMAPPassword decrypts the profile's saved password. It returns ""
// when there is none or it cannot be read, which the caller then reports as
// a missing password.
func (s *Server) storedIMAPPassword(ctx context.Context, profileID int64) string {
	if s.Secret == nil {
		return ""
	}
	blob, err := s.Store.GetIMAPProfilePassword(ctx, profileID)
	if err != nil || len(blob) == 0 {
		return ""
	}
	plain, err := s.Secret.Decrypt(blob)
	if err != nil {
		return ""
	}
	return string(plain)
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
