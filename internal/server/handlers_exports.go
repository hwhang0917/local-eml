package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/hwhang0917/local-eml/internal/exporter"
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

	exportID := newImportID()
	sourceName := fmt.Sprintf("s3://%s/%s", body.Bucket, body.Prefix)
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         exportID,
		SourceKind: "s3-export",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create export: "+err.Error(), http.StatusInternalServerError)
		return
	}

	cfg := exporter.S3Config{
		AccessKeyID:     body.AccessKeyID,
		SecretAccessKey: body.SecretAccessKey,
		SessionToken:    body.SessionToken,
		Region:          body.Region,
		Bucket:          body.Bucket,
		Prefix:          body.Prefix,
	}

	slog.Info("export requested",
		slog.String("export_id", exportID),
		slog.String("kind", "s3"),
		slog.String("bucket", body.Bucket),
		slog.String("prefix", body.Prefix),
		slog.String("region", body.Region),
		slog.Bool("static_creds", body.AccessKeyID != ""),
	)

	go s.runExport(exportID, cfg)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"export_id": exportID,
		"kind":      "s3-export",
	})
}

func (s *Server) runExport(id string, cfg exporter.S3Config) {
	ctx, release := s.Canceller.Register(context.Background(), id)
	defer release()
	_ = s.Store.UpdateImportStatus(ctx, id, "running", false)

	log := slog.Default().With(
		slog.String("export_id", id), slog.String("kind", "s3-export"))

	job := s.Exporter.NewS3Job(id, cfg)
	job.Logger = log

	defer func() {
		if rec := recover(); rec != nil {
			log.Error("export job panic", slog.Any("panic", rec))
			_ = s.Store.UpdateImportStatus(context.Background(), id, "error", true)
			s.Hub.Publish(id, importer.Event{Type: "error", Message: fmt.Sprint(rec)})
			s.Hub.Close(id)
		}
	}()

	job.Run(ctx)
}
