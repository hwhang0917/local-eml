package exporter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A folder backup is the S3 export's layout on a local or mounted disk: one
// "<sha>.eml" per message under emails/, plus the database snapshot. Files are
// named by content, so a run only copies what the folder doesn't have yet and
// a backup of 100,000 messages costs seconds once the first run is done.
// Nothing is ever deleted from the folder: mail deleted from the library stays
// in the backup, which is the point of having one.

// BackupEmailsDir holds the .eml files inside a backup folder.
const BackupEmailsDir = "emails"

const backupReadme = `This folder is a Local Eml backup.

emails/        every message, one .eml file each, named by its SHA-256
local-eml.db   stars, categories, settings and profiles (no passwords)

To restore on a new computer:
1. In Local Eml, go to Import and drop the "emails" folder onto the page.
2. Go to Settings > Restore from backup and choose local-eml.db.

Local Eml adds to this folder on every backup and never deletes from it.
`

// BackupResult is what one run did.
type BackupResult struct {
	Added   int           `json:"added"`
	Total   int           `json:"total"`
	Failed  int           `json:"failed"`
	Elapsed time.Duration `json:"-"`
}

// CheckBackupDir makes sure dir is an absolute path Local Eml can write to,
// creating it if needed.
func CheckBackupDir(dir string) error {
	if !filepath.IsAbs(dir) {
		return errors.New("the backup folder must be a full path, such as D:\\Backups\\Mail or /Volumes/Drive/Mail")
	}
	if err := os.MkdirAll(filepath.Join(dir, BackupEmailsDir), 0o700); err != nil {
		return fmt.Errorf("can't create the backup folder: %w", err)
	}
	probe, err := os.CreateTemp(dir, ".local-eml-write-test-")
	if err != nil {
		return fmt.Errorf("can't write to the backup folder: %w", err)
	}
	probe.Close()
	return os.Remove(probe.Name())
}

// BackupToFolder copies every message the folder lacks, then replaces the
// database snapshot. A message that fails to copy is counted and skipped; the
// run fails only when the folder itself is unusable.
func (e *Exporter) BackupToFolder(ctx context.Context, dir string) (BackupResult, error) {
	start := time.Now()
	var res BackupResult
	if err := CheckBackupDir(dir); err != nil {
		return res, err
	}
	emailsDir := filepath.Join(dir, BackupEmailsDir)

	have := map[string]bool{}
	ents, err := os.ReadDir(emailsDir)
	if err != nil {
		return res, err
	}
	for _, de := range ents {
		if name, ok := strings.CutSuffix(de.Name(), ".eml"); ok && len(name) == shaHexLen {
			have[name] = true
		}
	}

	entries, err := e.Store.AllExportEntries(ctx)
	if err != nil {
		return res, fmt.Errorf("list emails: %w", err)
	}
	res.Total = len(entries)
	for _, em := range entries {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		if have[em.SHA256] {
			continue
		}
		if err := copyAtomic(e.Paths.BlobFor(em.SHA256), filepath.Join(emailsDir, s3ObjectName(em.SHA256))); err != nil {
			slog.Warn("backup copy failed", slog.String("sha256", em.SHA256), slog.String("err", err.Error()))
			res.Failed++
			continue
		}
		res.Added++
	}

	// The snapshot goes to a temp name in the folder and is renamed over the
	// old one, so an interrupted run never leaves a half-written database.
	tmp := filepath.Join(dir, "."+DBObjectName+".tmp")
	_ = os.Remove(tmp)
	if err := e.Store.SnapshotTo(ctx, tmp); err != nil {
		_ = os.Remove(tmp)
		return res, fmt.Errorf("snapshot db: %w", err)
	}
	if err := os.Rename(tmp, filepath.Join(dir, DBObjectName)); err != nil {
		_ = os.Remove(tmp)
		return res, fmt.Errorf("save db snapshot: %w", err)
	}
	_ = os.WriteFile(filepath.Join(dir, "README.txt"), []byte(backupReadme), 0o600)

	res.Elapsed = time.Since(start)
	return res, nil
}

// copyAtomic copies src to dst through a temp file, so a crash mid-copy leaves
// no truncated .eml that a later run would mistake for a finished one.
func copyAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if st, err := in.Stat(); err == nil {
		_ = os.Chtimes(tmp, st.ModTime(), st.ModTime())
	}
	return os.Rename(tmp, dst)
}
