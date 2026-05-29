package exporter

import (
	"fmt"
	"path"
	"strings"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

type Exporter struct {
	Store *store.Store
	Paths paths.Paths
	Hub   *importer.Hub
}

// objectName returns a unique, safe name for the EML inside a ZIP or S3 bucket.
// Format: "<sha[:8]>_<sanitized-filename>" so different emails sharing the same
// original filename never collide, and content-addressed dedup is obvious.
func objectName(sha, originalName string) string {
	clean := sanitizeFilename(originalName)
	if clean == "" {
		return fmt.Sprintf("%s.eml", sha[:short(sha)])
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".eml") {
		clean += ".eml"
	}
	return fmt.Sprintf("%s_%s", sha[:short(sha)], clean)
}

func short(s string) int {
	if len(s) < 8 {
		return len(s)
	}
	return 8
}

func sanitizeFilename(name string) string {
	name = path.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.TrimSpace(name)
	if name == "." || name == ".." {
		return ""
	}
	return name
}
