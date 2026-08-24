package exporter

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
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

// DBObjectName is the archive/bucket entry for the metadata snapshot. The
// .eml blobs alone lose stars, categories, threads and import history — the
// SQLite database ties them together, so every export bundles a copy. The
// IMAP passwords inside are AES-GCM ciphertext; the key never leaves
// ~/.local-eml/keys, so the snapshot alone leaks no credentials.
const DBObjectName = "local-eml.db"

// snapshotDB writes a consistent copy of the live database to a temp file and
// returns its path plus a cleanup func.
func (e *Exporter) snapshotDB(ctx context.Context) (string, func(), error) {
	dir, err := os.MkdirTemp("", "local-eml-export-")
	if err != nil {
		return "", nil, err
	}
	p := filepath.Join(dir, DBObjectName)
	if err := e.Store.SnapshotTo(ctx, p); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("snapshot db: %w", err)
	}
	return p, func() { os.RemoveAll(dir) }, nil
}

// zipObjectName returns a flat, collision-resistant name for ZIP entries:
// "<sha[:8]>_<sanitized-basename>". Inside a zip archive we lose any directory
// structure anyway, so the short-SHA prefix guarantees two emails sharing a
// basename don't overwrite each other on extract.
func zipObjectName(sha, originalName string) string {
	clean := sanitizeFilename(originalName)
	if clean == "" {
		return fmt.Sprintf("%s.eml", sha[:short(sha)])
	}
	if !strings.HasSuffix(strings.ToLower(clean), ".eml") {
		clean += ".eml"
	}
	return fmt.Sprintf("%s_%s", sha[:short(sha)], clean)
}

// s3ObjectName returns a deterministic, content-addressed S3 object key for an
// email: just "<sha>.eml". The same email always lands at the same key, so
// re-running an export is a true no-op once the bucket is populated. We use
// the full SHA-256 (not a short prefix) so the key doubles as a strong
// content fingerprint that the user can verify with `aws s3 ls`.
func s3ObjectName(sha string) string {
	return sha + ".eml"
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
