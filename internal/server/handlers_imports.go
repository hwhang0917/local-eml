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
	"strings"

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

func (s *Server) runJob(importID, kind string, src importer.Source, cleanup func()) {
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
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Username string `json:"username"`
		Password string `json:"password"`
		Folder   string `json:"folder"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	body.Host = strings.TrimSpace(body.Host)
	body.Username = strings.TrimSpace(body.Username)
	body.Folder = strings.TrimSpace(body.Folder)
	if body.Host == "" || body.Username == "" || body.Password == "" {
		http.Error(w, "host, username and password are required", http.StatusBadRequest)
		return
	}

	folder := body.Folder
	if folder == "" {
		folder = "INBOX"
	}
	importID := newImportID()
	sourceName := fmt.Sprintf("imap://%s@%s/%s", body.Username, body.Host, folder)
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: "imap",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	src := importer.NewIMAPSource(importer.IMAPConfig{
		Host:     body.Host,
		Port:     body.Port,
		Username: body.Username,
		Password: body.Password,
		Folder:   body.Folder,
	})

	slog.Info("import requested",
		slog.String("import_id", importID),
		slog.String("kind", "imap"),
		slog.String("host", body.Host),
		slog.Int("port", body.Port),
		slog.String("username", body.Username),
		slog.String("folder", folder),
	)
	go s.runJob(importID, "imap", src, func() {})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      "imap",
	})
}
