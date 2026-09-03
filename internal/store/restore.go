package store

import (
	"context"
	"fmt"
)

// RestoreSummary reports how many rows a metadata restore touched.
type RestoreSummary struct {
	Emails       int64 `json:"emails"`
	Categories   int64 `json:"categories"`
	Settings     int64 `json:"settings"`
	ImapProfiles int64 `json:"imap_profiles"`
	S3Profiles   int64 `json:"s3_profiles"`
}

// RestoreMetadata merges metadata from an exported database snapshot into the
// live database. It is additive, keyed by stable identities (email sha256,
// category color, profile name), so it never deletes anything and re-running
// it is harmless:
//
//   - category names/positions are taken from the backup
//   - stars and category assignments are applied to emails that exist locally
//     (import the .eml files first — emails only in the backup are untouched)
//   - settings are overwritten with the backup's values
//   - IMAP/S3 profiles are added unless a profile with the same name exists
//
// Encrypted IMAP passwords are never restored: they were scrubbed at export
// because the AES key never leaves the exporting machine. Restored profiles
// come back with sync disabled and need their password re-entered.
func (s *Store) RestoreMetadata(ctx context.Context, snapshotPath string) (RestoreSummary, error) {
	var sum RestoreSummary

	// ATTACH is connection-scoped, so pin one pool connection for the whole
	// merge and detach before releasing it.
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return sum, err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "ATTACH DATABASE ? AS backup", snapshotPath); err != nil {
		return sum, fmt.Errorf("attach backup: %w", err)
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "DETACH DATABASE backup")

	// Snapshots from before the spam/phishing flag existed lack the column;
	// referencing it would fail the whole emails step.
	var hasFlag int
	if err := conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM backup.pragma_table_info('emails') WHERE name = 'flag'`).Scan(&hasFlag); err != nil {
		return sum, fmt.Errorf("inspect backup schema: %w", err)
	}
	flagCol := ""
	if hasFlag > 0 {
		flagCol = "flag = b.flag,"
	}

	steps := []struct {
		dst  *int64
		name string
		sql  string
	}{
		{&sum.Categories, "categories",
			`UPDATE categories SET name = b.name, position = b.position
			 FROM backup.categories b
			 WHERE categories.color = b.color
			   AND (categories.name <> b.name OR categories.position <> b.position)`},
		// category_id is remapped through the color, the stable identity —
		// row ids are seeded and should match, but colors are the contract.
		{&sum.Emails, "emails",
			`UPDATE emails SET
			   starred = b.starred, ` + flagCol + `
			   category_id = (SELECT c.id FROM categories c
			                  JOIN backup.categories bc ON bc.color = c.color
			                  WHERE bc.id = b.category_id)
			 FROM backup.emails b
			 WHERE emails.sha256 = b.sha256`},
		{&sum.Settings, "settings",
			`INSERT OR REPLACE INTO settings (key, value)
			 SELECT key, value FROM backup.settings`},
		{&sum.ImapProfiles, "imap_profiles",
			`INSERT OR IGNORE INTO imap_profiles
			   (name, host, port, username, folder, created_at, updated_at)
			 SELECT name, host, port, username, folder, created_at, updated_at
			 FROM backup.imap_profiles`},
		{&sum.S3Profiles, "s3_profiles",
			`INSERT OR IGNORE INTO s3_profiles
			   (name, bucket, prefix, region, access_key_id, created_at, updated_at)
			 SELECT name, bucket, prefix, region, access_key_id, created_at, updated_at
			 FROM backup.s3_profiles`},
	}
	for _, st := range steps {
		res, err := conn.ExecContext(ctx, st.sql)
		if err != nil {
			return sum, fmt.Errorf("restore %s: %w", st.name, err)
		}
		n, _ := res.RowsAffected()
		*st.dst = n
	}
	return sum, nil
}
