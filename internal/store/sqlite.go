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

const schemaSQL = `
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
  starred INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_emails_sent_at ON emails(sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_emails_from ON emails(from_addr);

CREATE VIRTUAL TABLE IF NOT EXISTS emails_fts USING fts5(
  subject, from_addr, to_addrs, body_text,
  content='', tokenize='unicode61 remove_diacritics 2'
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
`

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, schemaSQL); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "emails", "starred", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx,
		`CREATE INDEX IF NOT EXISTS idx_emails_starred ON emails(starred) WHERE starred = 1`)
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
