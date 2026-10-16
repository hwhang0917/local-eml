package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

// Job kinds, as stored on the imports row and shown in the job list. The
// exporter's kinds end in "-export" so the SPA can tell the two lists apart.
const (
	kindZip      = "zip"
	kindMbox     = "mbox"
	kindPst      = "pst"
	kindFile     = "file"
	kindDir      = "dir"
	kindS3       = "s3"
	kindIMAP     = "imap"
	kindIMAPSync = "imap-sync"
	kindResync   = "resync"
	kindS3Export = "s3-export"
)

// jobIDBytes sizes the random job id: 12 bytes is 24 hex characters, plenty
// to never collide within one library.
const jobIDBytes = 12

func newJobID() string {
	b := make([]byte, jobIDBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// queueJob creates the imports row every job starts from and returns its id.
func (s *Server) queueJob(ctx context.Context, kind, sourceName string) (string, error) {
	id := newJobID()
	err := s.Store.CreateImport(ctx, store.Import{
		ID:         id,
		SourceKind: kind,
		SourceName: sourceName,
		Status:     store.ImportQueued,
	})
	if err != nil {
		return "", fmt.Errorf("create import: %w", err)
	}
	return id, nil
}

// writeJobAccepted is the 202 every job endpoint answers with. extra lets an
// endpoint add fields (the resync reports its total).
func writeJobAccepted(w http.ResponseWriter, idKey, id, kind string, extra map[string]any) {
	body := map[string]any{idKey: id, "kind": kind}
	for k, v := range extra {
		body[k] = v
	}
	writeJSON(w, http.StatusAccepted, body)
}

// runJob drives one import to completion. cleanup runs when the job ends
// whatever happened; afterDone runs only after a normal finish. Either may be
// nil.
func (s *Server) runJob(id, kind string, src importer.Source, cleanup, afterDone func()) {
	if cleanup != nil {
		defer cleanup()
	}
	ctx, release := s.Canceller.Register(context.Background(), id)
	defer release()
	_ = s.Store.UpdateImportStatus(ctx, id, store.ImportRunning)

	job := &importer.Job{
		Importer: s.Importer,
		Hub:      s.Hub,
		Store:    s.Store,
		ID:       id,
		Logger:   slog.Default().With(slog.String("import_id", id), slog.String("kind", kind)),
	}
	defer s.failJobOnPanic(job)

	job.RunSource(ctx, src)
	if afterDone != nil {
		afterDone()
	}
}

// failJobOnPanic is deferred by every job runner: a panic inside a source or
// the uploader must still leave the row terminal and the stream closed, or
// the SPA spins forever.
func (s *Server) failJobOnPanic(job *importer.Job) {
	if rec := recover(); rec != nil {
		job.Log().Error("job panic", slog.Any("panic", rec))
		job.Fail("", fmt.Errorf("%v", rec))
		s.Hub.Close(job.ID)
	}
}

func (s *Server) handleCancelJob(w http.ResponseWriter, r *http.Request) {
	if !s.Canceller.Cancel(chi.URLParam(r, "id")) {
		http.Error(w, "job not running", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
