package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

func (s *Server) handleExportZip(w http.ResponseWriter, r *http.Request) {
	filename := fmt.Sprintf("local-eml-export-%s.zip", time.Now().Format("20060102-150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if _, _, err := s.Exporter.WriteZip(r.Context(), w); err != nil {
		slog.Error("zip export failed", slog.String("err", err.Error()))
		// Headers already sent; only thing we can do is stop writing.
		return
	}
}

func (s *Server) handleExportS3(w http.ResponseWriter, r *http.Request) {
	cfg, ok := decodeS3Config(w, r)
	if !ok {
		return
	}
	id, err := s.queueJob(r.Context(), kindS3Export, cfg.Label())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s3Logger(cfg).Info("export requested", slog.String("export_id", id), slog.String("kind", kindS3Export))
	go s.runExport(id, cfg)
	writeJobAccepted(w, "export_id", id, kindS3Export, nil)
}

func (s *Server) runExport(id string, cfg importer.S3Config) {
	ctx, release := s.Canceller.Register(context.Background(), id)
	defer release()
	_ = s.Store.UpdateImportStatus(ctx, id, store.ImportRunning)

	job := s.Exporter.NewS3Job(id, cfg)
	job.Logger = slog.Default().With(slog.String("export_id", id), slog.String("kind", kindS3Export))
	defer s.failJobOnPanic(&job.Job)

	job.Run(ctx)
}
