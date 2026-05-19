package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

var ErrEmailNotFound = errors.New("email not found")

func (s *Store) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT t.name, COUNT(et.email_id)
		FROM tags t
		LEFT JOIN email_tags et ON et.tag_id = t.id
		GROUP BY t.id
		ORDER BY t.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Tag{}
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.Name, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) GetTagsForEmail(ctx context.Context, sha string) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT t.name FROM tags t
		JOIN email_tags et ON et.tag_id = t.id
		JOIN emails e ON e.id = et.email_id
		WHERE e.sha256 = ?
		ORDER BY t.name COLLATE NOCASE`, sha)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// TagsForEmailIDs returns a map of email id → sorted tag names, used to batch-
// hydrate list responses without N+1 queries.
func (s *Store) TagsForEmailIDs(ctx context.Context, ids []int64) (map[int64][]string, error) {
	out := map[int64][]string{}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT et.email_id, t.name FROM tags t
		JOIN email_tags et ON et.tag_id = t.id
		WHERE et.email_id IN (`+placeholders+`)
		ORDER BY t.name COLLATE NOCASE`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = append(out[id], name)
	}
	return out, rows.Err()
}

// AddTagToEmail creates the tag if needed and links it to the email.
// Idempotent: re-adding the same tag returns nil.
func (s *Store) AddTagToEmail(ctx context.Context, sha, name string) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var emailID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM emails WHERE sha256 = ?`, sha).Scan(&emailID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmailNotFound
		}
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO tags (name) VALUES (?)`, name); err != nil {
		return err
	}
	var tagID int64
	if err := tx.QueryRowContext(ctx,
		`SELECT id FROM tags WHERE name = ?`, name).Scan(&tagID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO email_tags (email_id, tag_id) VALUES (?, ?)`,
		emailID, tagID); err != nil {
		return err
	}
	return tx.Commit()
}

// RemoveTagFromEmail unlinks a tag from an email. Idempotent if the link was
// already absent; returns ErrEmailNotFound only when the email itself doesn't
// exist. The tag row is left in place even if no other emails reference it.
func (s *Store) RemoveTagFromEmail(ctx context.Context, sha, name string) error {
	res, err := s.DB.ExecContext(ctx, `
		DELETE FROM email_tags
		WHERE email_id = (SELECT id FROM emails WHERE sha256 = ?)
		  AND tag_id   = (SELECT id FROM tags   WHERE name   = ?)`, sha, name)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		return nil
	}
	var exists int
	err = s.DB.QueryRowContext(ctx,
		`SELECT 1 FROM emails WHERE sha256 = ?`, sha).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrEmailNotFound
	}
	return err
}
