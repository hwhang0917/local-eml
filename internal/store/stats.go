package store

import (
	"context"
	"time"
)

type YearCount struct {
	Year  string `json:"year"`
	Count int    `json:"count"`
}

type SenderCount struct {
	From  string `json:"from"`
	Count int    `json:"count"`
}

type CategoryCount struct {
	CategoryID int64 `json:"category_id"`
	Count      int   `json:"count"`
}

type Stats struct {
	TotalCount      int             `json:"total_count"`
	TotalBytes      int64           `json:"total_bytes"`
	StarredCount    int             `json:"starred_count"`
	AttachmentCount int             `json:"attachment_count"`
	UndatedCount    int             `json:"undated_count"`
	PerYear         []YearCount     `json:"per_year"`
	TopSenders      []SenderCount   `json:"top_senders"`
	PerCategory     []CategoryCount `json:"per_category"`
}

// Stats aggregates over the emails table only; it never touches blobs.
func (s *Store) Stats(ctx context.Context) (*Stats, error) {
	st := &Stats{
		PerYear:     []YearCount{},
		TopSenders:  []SenderCount{},
		PerCategory: []CategoryCount{},
	}

	if err := s.DB.QueryRowContext(ctx, `
		SELECT COUNT(*), IFNULL(SUM(size_bytes), 0), IFNULL(SUM(starred), 0),
			IFNULL(SUM(has_attachments), 0), IFNULL(SUM(sent_at = 0), 0)
		FROM emails`).Scan(
		&st.TotalCount, &st.TotalBytes, &st.StarredCount,
		&st.AttachmentCount, &st.UndatedCount,
	); err != nil {
		return nil, err
	}

	rows, err := s.DB.QueryContext(ctx, `
		SELECT strftime('%Y', sent_at, 'unixepoch'), COUNT(*)
		FROM emails WHERE sent_at > 0 GROUP BY 1 ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var yc YearCount
		if err := rows.Scan(&yc.Year, &yc.Count); err != nil {
			return nil, err
		}
		st.PerYear = append(st.PerYear, yc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// ponytail: groups by the raw From header, so "Alice <a@x>" and "a@x" count
	// separately; normalize the address in SQL if that ever matters.
	rows, err = s.DB.QueryContext(ctx, `
		SELECT from_addr, COUNT(*) c FROM emails
		WHERE from_addr != '' GROUP BY from_addr ORDER BY c DESC, from_addr LIMIT 10`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sc SenderCount
		if err := rows.Scan(&sc.From, &sc.Count); err != nil {
			return nil, err
		}
		st.TopSenders = append(st.TopSenders, sc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.DB.QueryContext(ctx, `
		SELECT category_id, COUNT(*) FROM emails
		WHERE category_id IS NOT NULL GROUP BY category_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var cc CategoryCount
		if err := rows.Scan(&cc.CategoryID, &cc.Count); err != nil {
			return nil, err
		}
		st.PerCategory = append(st.PerCategory, cc)
	}
	return st, rows.Err()
}

// CountEmailsByDay buckets sent_at into local calendar days between from and
// to (unix seconds, inclusive), keyed YYYY-MM-DD. Bucketing happens in Go via
// time.Local — not sqlite's 'localtime', which need not agree with Go's zone —
// so counts always match dayBound-filtered list queries. Unlike PerYear's UTC
// buckets, a calendar cell must equal what clicking it lists. A month is a few
// thousand ints at most.
func (s *Store) CountEmailsByDay(ctx context.Context, from, to int64) (map[string]int, error) {
	out := map[string]int{}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT sent_at FROM emails WHERE sent_at != 0 AND sent_at BETWEEN ? AND ?`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[time.Unix(v, 0).In(time.Local).Format(time.DateOnly)]++
	}
	return out, rows.Err()
}
