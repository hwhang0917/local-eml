package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
	} {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	s := &Store{DB: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

// SnapshotTo writes a consistent point-in-time copy of the database to path
// via VACUUM INTO, which is safe against a live WAL-mode database (a plain
// file copy is not). The target must not already exist.
//
// Encrypted IMAP passwords are scrubbed from the copy: the AES key never
// leaves this machine, so on any other install the ciphertext is unrecoverable
// noise — exporting it is pure liability. Sync flags are cleared with them so
// a restored profile doesn't try to sync without credentials.
func (s *Store) SnapshotTo(ctx context.Context, path string) error {
	if _, err := s.DB.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return err
	}
	snap, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer snap.Close()
	_, err = snap.ExecContext(ctx,
		`UPDATE imap_profiles SET encrypted_password = NULL, sync_enabled = 0`)
	return err
}

const schemaSQL = `
-- Finder's model: a fixed set of colours, one row each, seeded in migrate and
-- never created or deleted — only renamed. Colour is therefore the identity and
-- carries the UNIQUE, and an empty name means "show the colour's own name",
-- which keeps the default localizable without the database knowing a locale.
-- Declared before emails so the foreign key below always resolves.
CREATE TABLE IF NOT EXISTS categories (
  id INTEGER PRIMARY KEY,
  color TEXT UNIQUE NOT NULL,
  name TEXT NOT NULL DEFAULT '',
  position INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS emails (
  id INTEGER PRIMARY KEY,
  sha256 TEXT UNIQUE NOT NULL,
  filename TEXT,
  subject TEXT,
  from_addr TEXT,
  to_addrs TEXT,
  cc_addrs TEXT,
  message_id TEXT,
  sent_at INTEGER,
  received_at INTEGER,
  size_bytes INTEGER,
  has_attachments INTEGER,
  attachment_count INTEGER,
  imported_at INTEGER,
  starred INTEGER NOT NULL DEFAULT 0,
  category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_emails_sent_at ON emails(sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_emails_from ON emails(from_addr);

CREATE VIRTUAL TABLE IF NOT EXISTS emails_fts USING fts5(
  subject, from_addr, to_addrs, body_text,
  content='', tokenize='unicode61 remove_diacritics 2'
);

-- emails_fts is contentless, so SQLite refuses a plain DELETE and the documented
-- 'delete' command needs the original body_text, which a contentless index does
-- not keep. Deleting a row therefore has to orphan its index entry, which is
-- harmless only while its rowid is never handed to a different email: a
-- duplicate rowid insert into a contentless index succeeds silently, and the
-- old terms would then resolve to the new message. Plain INTEGER PRIMARY KEY
-- reuses max(id)+1 after the highest row is deleted, so ids come from this
-- high-water mark instead and only ever move forward.
CREATE TABLE IF NOT EXISTS email_id_seq (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  next_id INTEGER NOT NULL
);

DROP TABLE IF EXISTS email_tags;
DROP TABLE IF EXISTS tags;

CREATE TABLE IF NOT EXISTS imports (
  id TEXT PRIMARY KEY,
  source_kind TEXT NOT NULL,
  source_name TEXT,
  status TEXT NOT NULL,
  total INTEGER DEFAULT 0,
  processed INTEGER DEFAULT 0,
  duplicates INTEGER DEFAULT 0,
  errors INTEGER DEFAULT 0,
  started_at INTEGER,
  finished_at INTEGER
);

CREATE TABLE IF NOT EXISTS import_errors (
  id INTEGER PRIMARY KEY,
  import_id TEXT NOT NULL,
  path TEXT,
  message TEXT,
  FOREIGN KEY (import_id) REFERENCES imports(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS imap_profiles (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  host TEXT NOT NULL,
  port INTEGER,
  username TEXT NOT NULL,
  folder TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS s3_profiles (
  id INTEGER PRIMARY KEY,
  name TEXT UNIQUE NOT NULL,
  bucket TEXT NOT NULL,
  prefix TEXT,
  region TEXT,
  access_key_id TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- User-tunable knobs the UI can change at runtime (e.g. the IMAP poll
-- interval), as opposed to env vars, which need a restart.
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "emails", "starred", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_emails_starred ON emails(starred) WHERE starred = 1`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "emails", "chosung_text", "TEXT"); err != nil {
		return err
	}
	// flag: '' | 'spam' | 'phishing'. Flagged mail is hidden from listings and
	// served as plain text only; the partial index serves the settings page.
	if err := s.ensureColumn(ctx, "emails", "flag", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_emails_flag ON emails(flag) WHERE flag != ''`); err != nil {
		return err
	}
	// SQLite only permits ADD COLUMN with a REFERENCES clause when the default is
	// NULL, which it is here.
	if err := s.ensureColumn(ctx, "emails", "category_id",
		"INTEGER REFERENCES categories(id) ON DELETE SET NULL"); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_emails_category ON emails(category_id)
		 WHERE category_id IS NOT NULL`); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "emails", "thread_id", "TEXT"); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_emails_thread ON emails(thread_id)
		 WHERE thread_id IS NOT NULL`); err != nil {
		return err
	}
	if err := s.seedCategories(ctx); err != nil {
		return err
	}
	if err := s.backfillChosung(ctx); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx,
		`INSERT OR IGNORE INTO email_id_seq (id, next_id)
		 SELECT 1, IFNULL(MAX(id), 0) + 1 FROM emails`); err != nil {
		return err
	}
	for _, col := range []struct{ name, ddl string }{
		{"sync_enabled", "INTEGER NOT NULL DEFAULT 0"},
		{"uid_validity", "INTEGER"},
		{"last_uid", "INTEGER"},
		{"last_synced_at", "INTEGER"},
		{"encrypted_password", "BLOB"},
	} {
		if err := s.ensureColumn(ctx, "imap_profiles", col.name, col.ddl); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) backfillChosung(ctx context.Context) error {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, COALESCE(subject, ''), COALESCE(from_addr, '')
		 FROM emails WHERE chosung_text IS NULL`)
	if err != nil {
		return err
	}
	type pending struct {
		id   int64
		text string
	}
	var todo []pending
	for rows.Next() {
		var id int64
		var subject, from string
		if err := rows.Scan(&id, &subject, &from); err != nil {
			rows.Close()
			return err
		}
		todo = append(todo, pending{id: id, text: ToChosung(subject + " " + from)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, p := range todo {
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE emails SET chosung_text = ? WHERE id = ?`, p.text, p.id); err != nil {
			return err
		}
	}
	return nil
}

// UserVersion reads SQLite's PRAGMA user_version, used as a marker for one-time
// data repairs that need more than SQL (e.g. re-parsing blobs).
func (s *Store) UserVersion(ctx context.Context) (int, error) {
	var v int
	err := s.DB.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v)
	return v, err
}

func (s *Store) SetUserVersion(ctx context.Context, v int) error {
	// PRAGMA does not support parameter binding.
	_, err := s.DB.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", v))
	return err
}

func (s *Store) ensureColumn(ctx context.Context, table, column, ddl string) error {
	rows, err := s.DB.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx,
		fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl))
	return err
}
