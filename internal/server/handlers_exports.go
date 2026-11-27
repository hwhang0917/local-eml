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

const (
	exportFormatZip  = "zip"
	exportFormatMbox = "mbox"
)

// handleExportSelection downloads chosen messages as a .zip of .eml files or
// one .mbox. It is a form POST rather than fetch so the browser streams the
// file to disk instead of holding it in memory. Either the form lists shas
// (the library's checkbox selection, widened to whole conversations when
// threads=1), or it carries the library's filters and every match goes out.
func (s *Server) handleExportSelection(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	f := r.PostForm
	format := f.Get("format")
	if format != exportFormatZip && format != exportFormatMbox {
		http.Error(w, `format must be "zip" or "mbox"`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	var emails []store.Email
	var err error
	if shas := f["sha"]; len(shas) > 0 {
		if len(shas) > maxBulkSHAs {
			http.Error(w, "too many messages selected", http.StatusBadRequest)
			return
		}
		for _, sha := range shas {
			if !validSHA(sha) {
				http.Error(w, "invalid sha", http.StatusBadRequest)
				return
			}
		}
		if f.Get("threads") == "1" {
			if shas, err = s.Store.ExpandThreads(ctx, shas); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		emails, err = s.Store.EmailsBySHA(ctx, shas)
	} else {
		opts := listFilters(f)
		// Spam and phishing never leave through a library export.
		opts.FlaggedOnly = false
		emails, err = s.Store.ExportMatching(ctx, opts)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(emails) == 0 {
		http.Error(w, "no messages to export", http.StatusNotFound)
		return
	}

	filename := fmt.Sprintf("local-eml-%d-messages-%s.%s", len(emails), time.Now().Format("20060102-150405"), format)
	ctype := "application/zip"
	if format == exportFormatMbox {
		ctype = "application/mbox"
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")

	write := s.Exporter.WriteSelectionZip
	if format == exportFormatMbox {
		write = s.Exporter.WriteMbox
	}
	written, skipped, err := write(ctx, w, emails)
	if err != nil {
		slog.Error("selection export failed", slog.String("format", format), slog.String("err", err.Error()))
		return
	}
	slog.Info("selection export finished", slog.String("format", format),
		slog.Int("written", written), slog.Int("skipped", skipped))
}
