package store

import (
	"context"
	"errors"
)

// ExpandThreads returns shas plus every visible message that shares a
// conversation with one of them, so a bulk action on a grouped row reaches
// the whole conversation. Unknown shas are kept: the caller's write decides
// what to do with them. Quarantined members stay out, as they do in the list.
func (s *Store) ExpandThreads(ctx context.Context, shas []string) ([]string, error) {
	seen := make(map[string]bool, len(shas))
	out := make([]string, 0, len(shas))
	add := func(sha string) {
		if !seen[sha] {
			seen[sha] = true
			out = append(out, sha)
		}
	}
	stmt, err := s.DB.PrepareContext(ctx, `
		SELECT sha256 FROM emails
		WHERE thread_id = (SELECT thread_id FROM emails WHERE sha256 = ?)
		  AND `+notQuarantined)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	for _, sha := range shas {
		add(sha)
		rows, err := stmt.QueryContext(ctx, sha)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var member string
			if err := rows.Scan(&member); err != nil {
				rows.Close()
				return nil, err
			}
			add(member)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// SetEmailsRead sets the read state of every listed message in one
// transaction and returns how many rows it touched. Unknown shas are skipped
// rather than failing the batch: the list may be stale by a message or two.
func (s *Store) SetEmailsRead(ctx context.Context, shas []string, read bool) (int64, error) {
	return s.eachSHA(ctx, `UPDATE emails SET read = ? WHERE sha256 = ?`, shas, read)
}

// DeleteEmailsBySHA removes the rows of every listed message in one
// transaction and returns the shas it actually deleted, so the caller removes
// only those blobs. As with DeleteEmailBySHA, the contentless FTS entries
// stay behind and resolve to no row.
func (s *Store) DeleteEmailsBySHA(ctx context.Context, shas []string) ([]string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, `DELETE FROM emails WHERE sha256 = ?`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	deleted := make([]string, 0, len(shas))
	for _, sha := range shas {
		res, err := stmt.ExecContext(ctx, sha)
		if err != nil {
			return nil, err
		}
		if err := requireRow(res, ErrEmailNotFound); err != nil {
			if errors.Is(err, ErrEmailNotFound) {
				continue
			}
			return nil, err
		}
		deleted = append(deleted, sha)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return deleted, nil
}

// eachSHA runs query once per sha inside one transaction, with arg bound
// before the sha, and returns the total rows affected.
func (s *Store) eachSHA(ctx context.Context, query string, shas []string, arg any) (int64, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	var n int64
	for _, sha := range shas {
		res, err := stmt.ExecContext(ctx, arg, sha)
		if err != nil {
			return 0, err
		}
		k, _ := res.RowsAffected()
		n += k
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return n, nil
}
