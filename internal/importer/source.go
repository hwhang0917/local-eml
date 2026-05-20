package importer

import (
	"context"
	"io"
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

// SourceCloser is a Source that holds a resource (e.g. an open archive) which
// must be released after Scan. The driver closes it when the import finishes.
type SourceCloser interface {
	Source
	io.Closer
}
