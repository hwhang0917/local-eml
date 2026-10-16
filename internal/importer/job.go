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

// Job drives one import or export: it owns the progress stream and the
// imports row for ID. The exporter embeds it so both share one lifecycle.
type Job struct {
	Importer *Importer
	Hub      *Hub
	Store    *store.Store
	ID       string
	Logger   *slog.Logger
}

// Log returns the job's logger, or the default one when none was set.
func (j *Job) Log() *slog.Logger {
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

	log := j.Log().With(slog.String("source", src.Label()))
	start := time.Now()
	log.Info("import job started")

	j.Publish(Event{Type: EventStep, Phase: "Scanning " + src.Label()})
	items, err := src.Scan(ctx)
	if err != nil {
		log.Error("source scan failed", slog.String("err", err.Error()))
		j.Fail("", err)
		return
	}

	total := len(items)
	log.Info("source scan complete", slog.Int("total", total))
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.Publish(Event{Type: EventStep, Phase: fmt.Sprintf("Importing %d emails", total), Total: total})

	var processed, duplicates, errs int
	cancelled := false
	for i, it := range items {
		if ctx.Err() != nil {
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

	if cancelled {
		j.MarkCancelled(processed, total)
	} else {
		j.MarkDone(total)
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
	log := j.Log().With(slog.String("item", it.Name), slog.Int("idx", idx), slog.Int("total", total))
	rc, err := it.Open(ctx)
	if err != nil {
		log.Warn("item open failed", slog.String("err", err.Error()))
		j.RecordItemError(ctx, it.Name, err, idx, total)
		return false, false
	}
	defer rc.Close()

	res, err := j.Importer.ImportReader(ctx, rc, it.Name)
	if err != nil {
		log.Warn("item import failed", slog.String("err", err.Error()))
		j.RecordItemError(ctx, it.Name, err, idx, total)
		return false, false
	}
	j.RecordItem(ctx, it.Name, res.SHA256, res.Duplicate, idx, total)
	log.Debug("item processed",
		slog.String("sha256", res.SHA256), slog.Bool("duplicate", res.Duplicate))
	return true, res.Duplicate
}

// RecordItem counts one processed item and streams it.
func (j *Job) RecordItem(ctx context.Context, name, sha string, duplicate bool, idx, total int) {
	dup := 0
	if duplicate {
		dup = 1
	}
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, dup, 0)
	j.Publish(Event{Type: EventItem, Path: name, SHA256: sha,
		Duplicate: duplicate, Processed: idx, Total: total})
}

// RecordItemError counts one failed item, keeps its message for the error
// list and streams it.
func (j *Job) RecordItemError(ctx context.Context, name string, cause error, idx, total int) {
	_ = j.Store.RecordImportError(ctx, j.ID, name, cause.Error())
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
	j.Publish(Event{Type: EventItem, Path: name, Message: cause.Error(),
		Processed: idx, Total: total})
}

// Terminal writes use a fresh context: the job's own ctx is already cancelled
// when the user aborted, but the final state still has to be recorded.

// Fail ends the job before its items ran, e.g. when the source cannot be
// listed. phase names where it stopped and may be empty.
func (j *Job) Fail(phase string, cause error) {
	_ = j.Store.UpdateImportStatus(context.Background(), j.ID, store.ImportError)
	j.Publish(Event{Type: EventError, Phase: phase, Message: cause.Error()})
}

// MarkCancelled records a user abort after processed of total items.
func (j *Job) MarkCancelled(processed, total int) {
	j.Publish(Event{Type: EventStep, Phase: "Cancelled"})
	_ = j.Store.UpdateImportStatus(context.Background(), j.ID, store.ImportError)
	j.Publish(Event{Type: EventError, Phase: "Cancelled",
		Message: "cancelled", Processed: processed, Total: total})
}

// MarkDone records a completed run.
func (j *Job) MarkDone(total int) {
	j.Publish(Event{Type: EventStep, Phase: "Finalizing"})
	_ = j.Store.UpdateImportStatus(context.Background(), j.ID, store.ImportDone)
	j.Publish(Event{Type: EventDone, Processed: total, Total: total})
}

// Publish streams ev to whoever is watching this job.
func (j *Job) Publish(ev Event) {
	j.Hub.Publish(j.ID, ev)
}

// IsEML reports whether name carries the .eml extension, case-insensitively.
func IsEML(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".eml")
}
