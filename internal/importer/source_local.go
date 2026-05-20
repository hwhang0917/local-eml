package importer

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// localFilesSource reads files already on disk (multipart temp files).
// filterEML drops non-.eml names (used for directory uploads); single-file
// uploads pass filterEML=false so the chosen file is always imported.
type localFilesSource struct {
	label     string
	paths     []string
	names     []string
	filterEML bool
}

func NewLocalSource(label string, paths, names []string, filterEML bool) Source {
	return &localFilesSource{label: label, paths: paths, names: names, filterEML: filterEML}
}

func (s *localFilesSource) Label() string { return s.label }

func (s *localFilesSource) Scan(_ context.Context) ([]Item, error) {
	items := make([]Item, 0, len(s.names))
	for i, name := range s.names {
		if s.filterEML && !isEML(name) {
			continue
		}
		p := s.paths[i]
		items = append(items, Item{
			Name: name,
			Open: func(context.Context) (io.ReadCloser, error) { return os.Open(p) },
		})
	}
	return items, nil
}

// zipSource streams .eml entries out of a zip archive. The archive is opened
// in Scan and held until Close (called by the driver via io.Closer).
type zipSource struct {
	path string
	zr   *zip.ReadCloser
}

func NewZipSource(path string) *zipSource { return &zipSource{path: path} }

func (s *zipSource) Label() string { return "zip archive" }

func (s *zipSource) Scan(_ context.Context) ([]Item, error) {
	zr, err := zip.OpenReader(s.path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	s.zr = zr
	var items []Item
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isEML(f.Name) {
			continue
		}
		ze := f
		items = append(items, Item{
			Name: filepath.Base(ze.Name),
			Open: func(context.Context) (io.ReadCloser, error) { return ze.Open() },
		})
	}
	return items, nil
}

func (s *zipSource) Close() error {
	if s.zr != nil {
		return s.zr.Close()
	}
	return nil
}
