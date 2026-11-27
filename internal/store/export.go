package store

import (
	"cmp"
	"context"
	"slices"
	"strings"
)

// Export listings are oldest first: a bundle handed to someone else reads
// like a correspondence file, from the first message to the last.
const exportOrder = ` ORDER BY sent_at ASC, id ASC`

// ExportMatching returns every message the library would list for opts, with
// no paging and no grouping. Sort, Limit, Offset and GroupThreads are ignored.
func (s *Store) ExportMatching(ctx context.Context, opts ListOptions) ([]Email, error) {
	where, args := listWhere(opts)
	return queryAll(ctx, s.DB, scanEmailValue,
		`SELECT `+emailCols+` FROM emails `+where+exportOrder, args...)
}

// shaChunk keeps each IN (...) well under SQLite's bound-parameter limit.
const shaChunk = 500

// EmailsBySHA returns the listed messages that exist, oldest first. Unknown
// shas are skipped: the caller's list may be a moment stale. Quarantined mail
// stays out, as it does everywhere a selection can come from.
func (s *Store) EmailsBySHA(ctx context.Context, shas []string) ([]Email, error) {
	out := []Email{}
	for start := 0; start < len(shas); start += shaChunk {
		chunk := shas[start:min(start+shaChunk, len(shas))]
		args := make([]any, len(chunk))
		for i, sha := range chunk {
			args[i] = sha
		}
		marks := strings.TrimSuffix(strings.Repeat("?,", len(chunk)), ",")
		rows, err := queryAll(ctx, s.DB, scanEmailValue,
			`SELECT `+emailCols+` FROM emails WHERE sha256 IN (`+marks+`) AND `+notQuarantined, args...)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	sortChronological(out)
	return out, nil
}

func sortChronological(es []Email) {
	// Rows from different chunks arrive in no particular order.
	slices.SortStableFunc(es, func(a, b Email) int {
		if c := a.SentAt.Compare(b.SentAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}
