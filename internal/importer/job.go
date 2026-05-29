package importer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/hwhang0917/local-eml/internal/store"
)

type Job struct {
	Importer *Importer
	Hub      *Hub
	Store    *store.Store
	ID       string
	Logger   *slog.Logger
}

func (j *Job) logger() *slog.Logger {
	if j.Logger != nil {
		return j.Logger
	}
	return slog.Default()
}

func (j *Job) RunSource(ctx context.Context, src Source) {
	defer j.Hub.Close(j.ID)
	if c, ok := src.(io.Closer); ok {
		defer c.Close()
	}

	log := j.logger().With(slog.String("source", src.Label()))
	start := time.Now()
	log.Info("import job started")

	j.publish(Event{Type: "step", Phase: "Scanning " + src.Label()})
	items, err := src.Scan(ctx)
	if err != nil {
		log.Error("source scan failed", slog.String("err", err.Error()))
		_ = j.Store.UpdateImportStatus(ctx, j.ID, "error", true)
		j.publish(Event{Type: "error", Message: err.Error()})
		return
	}

	total := len(items)
	log.Info("source scan complete", slog.Int("total", total))
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.publish(Event{Type: "step", Phase: fmt.Sprintf("Importing %d emails", total), Total: total})

	var processed, duplicates, errs int
	cancelled := false
	for i, it := range items {
		if ctxDone(ctx) {
			cancelled = true
			log.Warn("import job cancelled",
				slog.Int("processed", processed), slog.Int("total", total))
			break
		}
		ok, dup := j.processItem(ctx, it, i+1, total)
		processed++
		if dup {
			duplicates++
		}
		if !ok {
			errs++
		}
	}

	// Use a fresh context for finalization writes: ctx is already cancelled
	// when the user aborted, but we still need to record the terminal state.
	finalCtx := context.Background()
	if cancelled {
		j.publish(Event{Type: "step", Phase: "Cancelled"})
		_ = j.Store.UpdateImportStatus(finalCtx, j.ID, "error", true)
		j.publish(Event{Type: "error", Phase: "Cancelled",
			Message: "cancelled", Processed: processed, Total: total})
	} else {
		j.publish(Event{Type: "step", Phase: "Finalizing"})
		_ = j.Store.UpdateImportStatus(finalCtx, j.ID, "done", true)
		j.publish(Event{Type: "done", Processed: total, Total: total})
	}

	log.Info("import job finished",
		slog.Int("processed", processed),
		slog.Int("duplicates", duplicates),
		slog.Int("errors", errs),
		slog.Int("total", total),
		slog.Duration("elapsed", time.Since(start)),
	)
}

func (j *Job) processItem(ctx context.Context, it Item, idx, total int) (ok, duplicate bool) {
	log := j.logger().With(slog.String("item", it.Name), slog.Int("idx", idx), slog.Int("total", total))
	rc, err := it.Open(ctx)
	if err != nil {
		log.Warn("item open failed", slog.String("err", err.Error()))
		j.recordItemError(ctx, it.Name, err, idx, total)
		return false, false
	}
	defer rc.Close()

	res, err := j.Importer.ImportReader(ctx, rc, it.Name)
	if err != nil {
		log.Warn("item import failed", slog.String("err", err.Error()))
		j.recordItemError(ctx, it.Name, err, idx, total)
		return false, false
	}
	dup := 0
	if res.Duplicate {
		dup = 1
	}
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, dup, 0)
	j.publish(Event{Type: "item", Path: it.Name, SHA256: res.SHA256,
		Duplicate: res.Duplicate, Processed: idx, Total: total})
	log.Debug("item processed",
		slog.String("sha256", res.SHA256), slog.Bool("duplicate", res.Duplicate))
	return true, res.Duplicate
}

func (j *Job) recordItemError(ctx context.Context, name string, cause error, idx, total int) {
	_ = j.Store.RecordImportError(ctx, j.ID, name, cause.Error())
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
	j.publish(Event{Type: "item", Path: name, Message: cause.Error(),
		Processed: idx, Total: total})
}

func (j *Job) publish(ev Event) {
	j.Hub.Publish(j.ID, ev)
}

func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func isEML(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".eml")
}
