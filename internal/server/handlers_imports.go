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

	go s.runJob(importID, kind, paths, names)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      kind,
	})
}

func (s *Server) runJob(importID, kind string, paths, names []string) {
	defer removeAll(paths)
	ctx := context.Background()
	_ = s.Store.UpdateImportStatus(ctx, importID, "running", false)

	job := &importer.Job{
		Importer: s.Importer,
		Hub:      s.Hub,
		Store:    s.Store,
		ID:       importID,
	}

	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("import job panic", "import_id", importID, "panic", rec)
			_ = s.Store.UpdateImportStatus(ctx, importID, "error", true)
			s.Hub.Publish(importID, importer.Event{Type: "error", Message: fmt.Sprint(rec)})
			s.Hub.Close(importID)
		}
	}()

	switch kind {
	case "zip":
		job.RunZip(ctx, paths[0])
	case "file":
		job.RunFile(ctx, paths[0], names[0])
	case "dir":
		job.RunDir(ctx, paths, names)
	}
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
