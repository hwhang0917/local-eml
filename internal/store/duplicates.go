package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Duplicates. The importer skips a message only when its bytes are identical,
// so the same email reaching the library by two roads gets stored twice: a
// Google Takeout .mbox adds X-Gmail-Labels, IMAP adds nothing, a .pst export
// rewrites headers. Copies of one email still share their Message-ID and Date,
// and that pair is what groups them here. Message-ID alone isn't enough: a few
// mailers reuse one for different messages, which the Date tells apart.

// dupKey normalises a Message-ID for grouping: angle brackets, spaces and case
// differ between exports of the same message.
const dupKey = `lower(trim(trim(message_id), '<>'))`

// dupGroups is every Message-ID/Date pair held by more than one visible
// message. Mail without a Message-ID or a date never groups.
const dupGroups = `SELECT ` + dupKey + ` AS k, sent_at AS t FROM emails
	WHERE message_id IS NOT NULL AND trim(message_id, ' <>') <> '' AND sent_at > 0 AND ` + notQuarantined + `
	GROUP BY k, t HAVING COUNT(*) > 1`

// DuplicateGroup is one email stored more than once, oldest import first.
// Keep is the copy a merge keeps unless told otherwise.
type DuplicateGroup struct {
	Key   string  `json:"key"`
	Items []Email `json:"items"`
	Keep  string  `json:"keep"`
}

// DuplicateSummary counts what a merge of everything would remove.
type DuplicateSummary struct {
	Groups int `json:"groups"`
	// Extra is the number of copies beyond the first in each group.
	Extra      int   `json:"extra"`
	ExtraBytes int64 `json:"extra_bytes"`
}

func (s *Store) DuplicateSummary(ctx context.Context) (DuplicateSummary, error) {
	var sum DuplicateSummary
	err := s.DB.QueryRowContext(ctx, `
		WITH g AS (`+dupGroups+`),
		m AS (
			SELECT e.size_bytes,
				ROW_NUMBER() OVER (PARTITION BY g.k, g.t ORDER BY e.id) AS rn
			FROM emails e JOIN g ON `+strings.ReplaceAll(dupKey, "message_id", "e.message_id")+` = g.k AND e.sent_at = g.t
			WHERE `+strings.ReplaceAll(notQuarantined, "flag", "e.flag")+`
		)
		SELECT (SELECT COUNT(*) FROM g),
			COUNT(*), COALESCE(SUM(size_bytes), 0) FROM m WHERE rn > 1`).
		Scan(&sum.Groups, &sum.Extra, &sum.ExtraBytes)
	return sum, err
}

// ListDuplicates returns one page of groups, newest email first.
func (s *Store) ListDuplicates(ctx context.Context, limit, offset int) ([]DuplicateGroup, error) {
	limit = ClampPageSize(limit)
	type key struct {
		k string
		t int64
	}
	keys, err := queryAll(ctx, s.DB, func(rs rowScanner) (key, error) {
		var k key
		return k, rs.Scan(&k.k, &k.t)
	}, dupGroups+` ORDER BY t DESC, k LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]DuplicateGroup, 0, len(keys))
	for _, k := range keys {
		items, err := s.duplicateCopies(ctx, s.DB, k.k, k.t)
		if err != nil {
			return nil, err
		}
		if len(items) < 2 {
			continue
		}
		out = append(out, DuplicateGroup{Key: groupKey(k.k, k.t), Items: items, Keep: items[0].SHA256})
	}
	return out, nil
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (s *Store) duplicateCopies(ctx context.Context, q querier, k string, t int64) ([]Email, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+emailCols+` FROM emails
		WHERE `+dupKey+` = ? AND sent_at = ? AND `+notQuarantined+` ORDER BY id`, k, t)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Email
	for rows.Next() {
		e, err := scanEmail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// groupKey names a group in the API: the normalised Message-ID and the date,
// so a merge request can't widen to a different message that shares the id.
func groupKey(k string, t int64) string { return fmt.Sprintf("%d %s", t, k) }

func parseGroupKey(g string) (string, int64, bool) {
	ts, k, ok := strings.Cut(g, " ")
	if !ok || k == "" {
		return "", 0, false
	}
	var t int64
	if _, err := fmt.Sscan(ts, &t); err != nil || t <= 0 {
		return "", 0, false
	}
	return k, t, true
}

// MergeRequest merges one group. Keep names the copy to keep; empty keeps the
// oldest import.
type MergeRequest struct {
	Group string `json:"group"`
	Keep  string `json:"keep,omitempty"`
}

// MergeDuplicates folds each group into one copy and returns the shas of the
// copies it removed, so the caller deletes their files. What the person did to
// any copy survives on the kept one: starred if any copy was, read if any was,
// and the kept copy's category, else the first category among the others.
// A group that no longer has two copies, or whose Keep isn't in it, is skipped.
func (s *Store) MergeDuplicates(ctx context.Context, reqs []MergeRequest) ([]string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var removed []string
	for _, r := range reqs {
		k, t, ok := parseGroupKey(r.Group)
		if !ok {
			continue
		}
		items, err := s.duplicateCopies(ctx, tx, k, t)
		if err != nil {
			return nil, err
		}
		if len(items) < 2 {
			continue
		}
		keep := items[0]
		if r.Keep != "" {
			found := false
			for _, e := range items {
				if e.SHA256 == r.Keep {
					keep, found = e, true
				}
			}
			if !found {
				continue
			}
		}
		starred, read := keep.Starred, keep.Read
		category := keep.CategoryID
		for _, e := range items {
			starred = starred || e.Starred
			read = read || e.Read
			if category == nil && e.CategoryID != nil {
				category = e.CategoryID
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE emails SET starred = ?, read = ?, category_id = ? WHERE id = ?`,
			starred, read, nullable(category), keep.ID); err != nil {
			return nil, err
		}
		for _, e := range items {
			if e.ID == keep.ID {
				continue
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM emails WHERE id = ?`, e.ID); err != nil {
				return nil, err
			}
			removed = append(removed, e.SHA256)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return removed, nil
}

// AllDuplicateGroups lists every group key, for "merge all".
func (s *Store) AllDuplicateGroups(ctx context.Context) ([]string, error) {
	return queryAll(ctx, s.DB, func(rs rowScanner) (string, error) {
		var k string
		var t int64
		err := rs.Scan(&k, &t)
		return groupKey(k, t), err
	}, dupGroups)
}
