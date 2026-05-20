package importer

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hwhang0917/local-eml/internal/store"
)

type Job struct {
	Importer *Importer
	Hub      *Hub
	Store    *store.Store
	ID       string
}

func (j *Job) RunSource(ctx context.Context, src Source) {
	defer j.Hub.Close(j.ID)
	if c, ok := src.(io.Closer); ok {
		defer c.Close()
	}

	j.publish(Event{Type: "step", Phase: "Scanning " + src.Label()})
	items, err := src.Scan(ctx)
	if err != nil {
		_ = j.Store.UpdateImportStatus(ctx, j.ID, "error", true)
		j.publish(Event{Type: "error", Message: err.Error()})
		return
	}

	total := len(items)
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.publish(Event{Type: "step", Phase: fmt.Sprintf("Importing %d emails", total), Total: total})

	for i, it := range items {
		if ctxDone(ctx) {
			break
		}
		j.processItem(ctx, it, i+1, total)
	}

	j.publish(Event{Type: "step", Phase: "Finalizing"})
	_ = j.Store.UpdateImportStatus(ctx, j.ID, "done", true)
	j.publish(Event{Type: "done", Processed: total, Total: total})
}

func (j *Job) processItem(ctx context.Context, it Item, idx, total int) {
	rc, err := it.Open(ctx)
	if err != nil {
		j.recordItemError(ctx, it.Name, err, idx, total)
		return
	}
	defer rc.Close()

	res, err := j.Importer.ImportReader(ctx, rc, it.Name)
	if err != nil {
		j.recordItemError(ctx, it.Name, err, idx, total)
		return
	}
	dup := 0
	if res.Duplicate {
		dup = 1
	}
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, dup, 0)
	j.publish(Event{Type: "item", Path: it.Name, SHA256: res.SHA256,
		Duplicate: res.Duplicate, Processed: idx, Total: total})
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
