package store

import (
	"context"
	"fmt"
)

// WipeLibrary deletes every email, its search index entries and the import
// history, and turns background sync off on every IMAP profile so the
// library doesn't refill on the next tick. Profiles, categories and settings
// are kept: they describe how the user works, not what their mail says.
// It returns how many emails were deleted. The .eml blobs on disk are the
// caller's job.
func (s *Store) WipeLibrary(ctx context.Context) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	steps := []string{
		`UPDATE imap_profiles SET ` + revokeIMAPSyncSet,
		`DELETE FROM import_errors`,
		`DELETE FROM imports`,
		// emails_fts is contentless, so per-row deletes can't clear it (see
		// the schema); 'delete-all' is the one command that can.
		`INSERT INTO emails_fts(emails_fts) VALUES('delete-all')`,
	}
	for _, q := range steps {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return 0, fmt.Errorf("wipe: %w", err)
		}
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM emails`)
	if err != nil {
		return 0, fmt.Errorf("wipe emails: %w", err)
	}
	n, _ := res.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	// Deleted rows linger in free pages and in the WAL until rewritten.
	// VACUUM rebuilds the file without them and the checkpoint truncates the
	// WAL, so the subjects and addresses are actually gone from disk.
	for _, q := range []string{`VACUUM`, `PRAGMA wal_checkpoint(TRUNCATE)`} {
		if _, err := s.DB.ExecContext(ctx, q); err != nil {
			return n, fmt.Errorf("compact after wipe: %w", err)
		}
	}
	return n, nil
}
