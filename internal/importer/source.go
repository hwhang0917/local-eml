package importer

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
)

// Item is one importable object. Open is lazy so bodies stream one at a time.
type Item struct {
	Name string
	Open func(ctx context.Context) (io.ReadCloser, error)
}

// Source enumerates .eml items from a provider (local upload, zip, S3, …).
type Source interface {
	// Label is a short human description for the "Scanning <label>" phase.
	Label() string
	// Scan returns candidate items already filtered to .eml.
	Scan(ctx context.Context) ([]Item, error)
}

// archiveItemName names the i-th (1-based) message of a single-file archive
// so progress and error lists can point at it: "<file>#00042.eml".
func archiveItemName(path string, i int) string {
	return fmt.Sprintf("%s#%05d.eml", filepath.Base(path), i)
}

// SourceCloser is a Source that holds a resource (e.g. an open archive) which
// must be released after Scan. The driver closes it when the import finishes.
type SourceCloser interface {
	Source
	io.Closer
}
