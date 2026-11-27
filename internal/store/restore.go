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
	return s.restoreMetadata(ctx, snapshotPath, nil)
}

// MergeSessionMetadata carries a kept temporary session's metadata into this
// library, after its mail has been imported. It differs from a backup restore
// in what it may overwrite, because the session ran beside this library
// rather than being a copy of it:
//
//   - stars, read marks, flags and categories are applied only to newSHAs,
//     the emails the merge just added. A message that was in both keeps this
//     library's marks, not the fresh "unread" it had in the session.
//   - category names are left alone: the session's are the seeded defaults,
//     and would undo the user's renames here
//   - settings are added only where this library has none
//   - profiles are added unless one with the same name exists, without
//     passwords (the session's key is deleted with it) and with sync off
func (s *Store) MergeSessionMetadata(ctx context.Context, sessionDB string, newSHAs []string) (RestoreSummary, error) {
	if newSHAs == nil {
		newSHAs = []string{}
	}
	return s.restoreMetadata(ctx, sessionDB, newSHAs)
}

// restoreMetadata is RestoreMetadata, or with onlySHAs non-nil the session
// merge described at MergeSessionMetadata.
func (s *Store) restoreMetadata(ctx context.Context, snapshotPath string, onlySHAs []string) (RestoreSummary, error) {
	var sum RestoreSummary
	session := onlySHAs != nil

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

	// Snapshots from before a column existed lack it; referencing it would
	// fail the whole emails step, so later columns are copied only when present.
	backupHas := func(table, col string) (bool, error) {
		var n int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pragma_table_info(?, 'backup') WHERE name = ?`, table, col).Scan(&n); err != nil {
			return false, fmt.Errorf("inspect backup schema: %w", err)
		}
		return n > 0, nil
	}
	optionalCols := ""
	for _, col := range []string{"flag", "read"} {
		ok, err := backupHas("emails", col)
		if err != nil {
			return sum, err
		}
		if ok {
			optionalCols += col + " = b." + col + ","
		}
	}
	s3Cols := ""
	for _, col := range []string{"endpoint", "path_style"} {
		ok, err := backupHas("s3_profiles", col)
		if err != nil {
			return sum, err
		}
		if ok {
			s3Cols += ", " + col
		}
	}

	emailFilter := ""
	settingsVerb := "INSERT OR REPLACE"
	if session {
		if _, err := conn.ExecContext(ctx, "CREATE TEMP TABLE merge_only (sha256 TEXT PRIMARY KEY)"); err != nil {
			return sum, fmt.Errorf("merge list: %w", err)
		}
		defer conn.ExecContext(context.WithoutCancel(ctx), "DROP TABLE temp.merge_only")
		for _, sha := range onlySHAs {
			if _, err := conn.ExecContext(ctx, "INSERT OR IGNORE INTO temp.merge_only (sha256) VALUES (?)", sha); err != nil {
				return sum, fmt.Errorf("merge list: %w", err)
			}
		}
		emailFilter = " AND emails.sha256 IN (SELECT sha256 FROM temp.merge_only)"
		settingsVerb = "INSERT OR IGNORE"
	}

	type step struct {
		dst  *int64
		name string
		sql  string
	}
	var steps []step
	if !session {
		steps = append(steps, step{&sum.Categories, "categories",
			`UPDATE categories SET name = b.name, position = b.position
			 FROM backup.categories b
			 WHERE categories.color = b.color
			   AND (categories.name <> b.name OR categories.position <> b.position)`})
	}
	steps = append(steps, []step{
		// category_id is remapped through the color, the stable identity —
		// row ids are seeded and should match, but colors are the contract.
		{&sum.Emails, "emails",
			`UPDATE emails SET
			   starred = b.starred, ` + optionalCols + `
			   category_id = (SELECT c.id FROM categories c
			                  JOIN backup.categories bc ON bc.color = c.color
			                  WHERE bc.id = b.category_id)
			 FROM backup.emails b
			 WHERE emails.sha256 = b.sha256` + emailFilter},
		{&sum.Settings, "settings",
			settingsVerb + ` INTO settings (key, value)
			 SELECT key, value FROM backup.settings`},
		{&sum.ImapProfiles, "imap_profiles",
			`INSERT OR IGNORE INTO imap_profiles
			   (name, host, port, username, folder, created_at, updated_at)
			 SELECT name, host, port, username, folder, created_at, updated_at
			 FROM backup.imap_profiles`},
		{&sum.S3Profiles, "s3_profiles",
			`INSERT OR IGNORE INTO s3_profiles
			   (name, bucket, prefix, region, access_key_id, created_at, updated_at` + s3Cols + `)
			 SELECT name, bucket, prefix, region, access_key_id, created_at, updated_at` + s3Cols + `
			 FROM backup.s3_profiles`},
	}...)
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
