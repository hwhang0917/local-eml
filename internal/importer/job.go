package importer

import (
	"archive/zip"
	"context"
	"path/filepath"
	"strings"

	"github.com/hwhang0917/local-eml/internal/store"
)

type Job struct {
	Importer *Importer
	Hub      *Hub
	Store    *store.Store
	ID       string
}

// RunFile imports a single .eml file. srcPath is a local temp path; the caller
// is responsible for cleaning it up after the job returns.
func (j *Job) RunFile(ctx context.Context, srcPath, name string) {
	defer j.Hub.Close(j.ID)
	_ = j.Store.SetImportTotal(ctx, j.ID, 1)
	j.publish(Event{Type: "start", Total: 1})
	j.processOne(ctx, srcPath, name, 1, 1, false)
	_ = j.Store.UpdateImportStatus(ctx, j.ID, "done", true)
	j.publish(Event{Type: "done", Processed: 1, Total: 1})
}

// RunDir imports each .eml file in srcPaths (filtered by name suffix).
// names[i] is the original filename for srcPaths[i].
func (j *Job) RunDir(ctx context.Context, srcPaths, names []string) {
	defer j.Hub.Close(j.ID)

	type entry struct{ path, name string }
	var entries []entry
	for i, n := range names {
		if isEML(n) {
			entries = append(entries, entry{srcPaths[i], n})
		}
	}
	total := len(entries)
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.publish(Event{Type: "start", Total: total})

	for i, e := range entries {
		if ctxDone(ctx) {
			break
		}
		j.processOne(ctx, e.path, e.name, i+1, total, false)
	}
	_ = j.Store.UpdateImportStatus(ctx, j.ID, "done", true)
	j.publish(Event{Type: "done", Processed: total, Total: total})
}

// RunZip iterates a zip archive at zipPath, importing each .eml entry streamed
// one at a time so memory stays bounded.
func (j *Job) RunZip(ctx context.Context, zipPath string) {
	defer j.Hub.Close(j.ID)

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		_ = j.Store.UpdateImportStatus(ctx, j.ID, "error", true)
		j.publish(Event{Type: "error", Message: "open zip: " + err.Error()})
		return
	}
	defer zr.Close()

	var entries []*zip.File
	for _, f := range zr.File {
		if !f.FileInfo().IsDir() && isEML(f.Name) {
			entries = append(entries, f)
		}
	}
	total := len(entries)
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.publish(Event{Type: "start", Total: total})

	for i, ze := range entries {
		if ctxDone(ctx) {
			break
		}
		j.processZipEntry(ctx, ze, i+1, total)
	}
	_ = j.Store.UpdateImportStatus(ctx, j.ID, "done", true)
	j.publish(Event{Type: "done", Processed: total, Total: total})
}

func (j *Job) processOne(ctx context.Context, srcPath, name string, idx, total int, _ bool) {
	res, err := j.Importer.ImportFile(ctx, srcPath, name)
	if err != nil {
		_ = j.Store.RecordImportError(ctx, j.ID, name, err.Error())
		_ = j.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
		j.publish(Event{Type: "item", Path: name, Message: err.Error(),
			Processed: idx, Total: total})
		return
	}
	dup := 0
	if res.Duplicate {
		dup = 1
	}
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, dup, 0)
	j.publish(Event{Type: "item", Path: name, SHA256: res.SHA256,
		Duplicate: res.Duplicate, Processed: idx, Total: total})
}

func (j *Job) processZipEntry(ctx context.Context, ze *zip.File, idx, total int) {
	name := filepath.Base(ze.Name)
	rc, err := ze.Open()
	if err != nil {
		_ = j.Store.RecordImportError(ctx, j.ID, ze.Name, err.Error())
		_ = j.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
		j.publish(Event{Type: "item", Path: ze.Name, Message: err.Error(),
			Processed: idx, Total: total})
		return
	}
	defer rc.Close()
	res, err := j.Importer.ImportReader(ctx, rc, name)
	if err != nil {
		_ = j.Store.RecordImportError(ctx, j.ID, ze.Name, err.Error())
		_ = j.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
		j.publish(Event{Type: "item", Path: ze.Name, Message: err.Error(),
			Processed: idx, Total: total})
		return
	}
	dup := 0
	if res.Duplicate {
		dup = 1
	}
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, dup, 0)
	j.publish(Event{Type: "item", Path: ze.Name, SHA256: res.SHA256,
		Duplicate: res.Duplicate, Processed: idx, Total: total})
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
