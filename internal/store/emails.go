package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type Email struct {
	ID              int64     `json:"id"`
	SHA256          string    `json:"sha256"`
	Filename        string    `json:"filename"`
	Subject         string    `json:"subject"`
	FromAddr        string    `json:"from"`
	ToAddrs         []string  `json:"to"`
	CcAddrs         []string  `json:"cc"`
	MessageID       string    `json:"message_id"`
	ThreadID        string    `json:"thread_id,omitempty"`
	SentAt          time.Time `json:"sent_at"`
	ReceivedAt      time.Time `json:"received_at"`
	SizeBytes       int64     `json:"size_bytes"`
	HasAttachments  bool      `json:"has_attachments"`
	AttachmentCount int       `json:"attachment_count"`
	ImportedAt      time.Time `json:"imported_at"`
	Starred         bool      `json:"starred"`
	// ThreadCount is only populated by grouped listings: how many messages the
	// row's conversation holds (1 for a singleton).
	ThreadCount int `json:"thread_count,omitempty"`
	// Nil when uncategorized. A pointer so the JSON omits the field entirely
	// rather than shipping a 0 the SPA would have to special-case.
	CategoryID *int64 `json:"category_id,omitempty"`
}

type EmailRow struct {
	Email
	BodyText string
}

var ErrDuplicate = errors.New("email already exists")

func (s *Store) InsertEmail(ctx context.Context, e EmailRow) (int64, error) {
	to, _ := json.Marshal(stringsOrEmpty(e.ToAddrs))
	cc, _ := json.Marshal(stringsOrEmpty(e.CcAddrs))
	hasAtt := 0
	if e.HasAttachments {
		hasAtt = 1
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Ids come from the high-water mark rather than SQLite's max(id)+1 so a
	// deleted row's number is never handed out again — see email_id_seq in the
	// schema for why the contentless FTS index depends on that.
	id, err := nextEmailID(ctx, tx)
	if err != nil {
		return 0, err
	}

	chosung := ToChosung(e.Subject + " " + e.FromAddr)
	_, err = tx.ExecContext(ctx, `
		INSERT INTO emails (id, sha256, filename, subject, from_addr, to_addrs, cc_addrs,
			message_id, thread_id, sent_at, received_at, size_bytes, has_attachments,
			attachment_count, imported_at, chosung_text)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, e.SHA256, e.Filename, e.Subject, e.FromAddr, string(to), string(cc),
		e.MessageID, nullIfEmpty(e.ThreadID), unixOrZero(e.SentAt), unixOrZero(e.ReceivedAt),
		e.SizeBytes, hasAtt, e.AttachmentCount, time.Now().Unix(), chosung,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicate
		}
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO emails_fts (rowid, subject, from_addr, to_addrs, body_text)
		VALUES (?,?,?,?,?)`,
		id, e.Subject, e.FromAddr, string(to), e.BodyText,
	); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func nextEmailID(ctx context.Context, tx *sql.Tx) (int64, error) {
	// MAX against the live table too, so a database written before the sequence
	// existed cannot hand out an id that is already taken.
	if _, err := tx.ExecContext(ctx, `
		UPDATE email_id_seq
		SET next_id = MAX(next_id, (SELECT IFNULL(MAX(id), 0) + 1 FROM emails))
		WHERE id = 1`); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRowContext(ctx,
		`SELECT next_id FROM email_id_seq WHERE id = 1`).Scan(&id); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE email_id_seq SET next_id = ? WHERE id = 1`, id+1); err != nil {
		return 0, err
	}
	return id, nil
}

// DeleteEmailBySHA removes the row for a message whose blob has gone missing.
// The contentless FTS index cannot be updated (see email_id_seq in the schema),
// so its entry is left behind; it resolves to no row and its id is never
// reissued, so searches stay correct.
func (s *Store) DeleteEmailBySHA(ctx context.Context, sha string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM emails WHERE sha256 = ?`, sha)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrEmailNotFound
	}
	return nil
}

func (s *Store) EmailExists(ctx context.Context, sha string) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM emails WHERE sha256 = ?`, sha).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) GetEmailBySHA(ctx context.Context, sha string) (*Email, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, sha256, filename, subject, from_addr, to_addrs, cc_addrs,
			message_id, COALESCE(thread_id, ''), sent_at, received_at, size_bytes,
			has_attachments, attachment_count, imported_at, starred, category_id
		FROM emails WHERE sha256 = ?`, sha)
	return scanEmail(row)
}

// ListThread returns every email sharing threadID, oldest first.
func (s *Store) ListThread(ctx context.Context, threadID string) ([]Email, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, sha256, filename, subject, from_addr, to_addrs, cc_addrs,
			message_id, COALESCE(thread_id, ''), sent_at, received_at, size_bytes,
			has_attachments, attachment_count, imported_at, starred, category_id
		FROM emails WHERE thread_id = ? ORDER BY sent_at ASC, id ASC`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Email{}
	for rows.Next() {
		e, err := scanEmail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

var ErrEmailNotFound = errors.New("email not found")

type ExportEntry struct {
	SHA256   string
	Filename string
}

func (s *Store) AllExportEntries(ctx context.Context) ([]ExportEntry, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT sha256, COALESCE(filename, '') FROM emails ORDER BY sent_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExportEntry{}
	for rows.Next() {
		var e ExportEntry
		if err := rows.Scan(&e.SHA256, &e.Filename); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// EmailAttachmentRow is the minimal projection the attachment-count backfill
// needs to decide whether a row must be re-parsed and rewritten.
type EmailAttachmentRow struct {
	ID              int64
	SHA256          string
	AttachmentCount int
}

func (s *Store) ListEmailAttachmentCounts(ctx context.Context) ([]EmailAttachmentRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, sha256, COALESCE(attachment_count, 0) FROM emails`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EmailAttachmentRow{}
	for rows.Next() {
		var r EmailAttachmentRow
		if err := rows.Scan(&r.ID, &r.SHA256, &r.AttachmentCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SetEmailAttachmentCount(ctx context.Context, id int64, count int) error {
	flag := 0
	if count > 0 {
		flag = 1
	}
	_, err := s.DB.ExecContext(ctx,
		`UPDATE emails SET has_attachments = ?, attachment_count = ? WHERE id = ?`,
		flag, count, id)
	return err
}

// ThreadBackfillRow is the minimal projection the thread-id backfill needs.
type ThreadBackfillRow struct {
	ID     int64
	SHA256 string
}

func (s *Store) ListEmailsMissingThreadID(ctx context.Context) ([]ThreadBackfillRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT id, sha256 FROM emails WHERE thread_id IS NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ThreadBackfillRow{}
	for rows.Next() {
		var r ThreadBackfillRow
		if err := rows.Scan(&r.ID, &r.SHA256); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) SetEmailThreadID(ctx context.Context, id int64, threadID string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE emails SET thread_id = ? WHERE id = ?`, nullIfEmpty(threadID), id)
	return err
}

func (s *Store) SetEmailStarred(ctx context.Context, sha string, starred bool) error {
	flag := 0
	if starred {
		flag = 1
	}
	res, err := s.DB.ExecContext(ctx,
		`UPDATE emails SET starred = ? WHERE sha256 = ?`, flag, sha)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrEmailNotFound
	}
	return nil
}

type ListOptions struct {
	Query       string
	StarredOnly bool
	Sort        string
	Order       string
	Limit       int
	Offset      int
	// From and To bound sent_at, in unix seconds; zero means unbounded. Both
	// ends are inclusive, so the caller decides where a day starts and ends.
	From int64
	To   int64
	// CategoryID restricts to one category; nil means any. Uncategorized wins
	// when both are set, since "no category" and "this category" can't both hold.
	CategoryID    *int64
	Uncategorized bool
	// GroupThreads collapses each conversation to its newest matching message,
	// with ThreadCount carrying the group size. Filters apply to members before
	// grouping, so a search shows every conversation containing a match.
	GroupThreads bool
}

func (s *Store) ListEmails(ctx context.Context, opts ListOptions) ([]Email, int, error) {
	if opts.Limit <= 0 || opts.Limit > 500 {
		opts.Limit = 50
	}
	sortCol := "sent_at"
	switch opts.Sort {
	case "from_addr", "subject", "size_bytes", "imported_at", "sent_at":
		sortCol = opts.Sort
	}
	order := "DESC"
	if strings.EqualFold(opts.Order, "asc") {
		order = "ASC"
	}

	args := []any{}
	conds := []string{}
	if HasJamo(opts.Query) {
		for _, term := range strings.Fields(opts.Query) {
			conds = append(conds, `chosung_text LIKE ?`)
			args = append(args, "%"+ToChosung(term)+"%")
		}
	} else if fts := buildFTSQuery(opts.Query); fts != "" {
		conds = append(conds, `id IN (SELECT rowid FROM emails_fts WHERE emails_fts MATCH ?)`)
		args = append(args, fts)
	}
	if opts.StarredOnly {
		conds = append(conds, `starred = 1`)
	}
	// A plain predicate on emails, never a JOIN — the COUNT(*) below shares this
	// WHERE, and a join would double-count.
	if opts.Uncategorized {
		conds = append(conds, `category_id IS NULL`)
	} else if opts.CategoryID != nil {
		conds = append(conds, `category_id = ?`)
		args = append(args, *opts.CategoryID)
	}
	// Messages with no parseable Date header store sent_at = 0, so any bound
	// drops them — which is what a date filter should do.
	if opts.From > 0 {
		conds = append(conds, `sent_at >= ?`)
		args = append(args, opts.From)
	}
	if opts.To > 0 {
		conds = append(conds, `sent_at <= ?`)
		args = append(args, opts.To)
	}
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	// The COALESCE is aliased so the grouped query's outer SELECT can re-list
	// these columns against the subquery.
	const cols = `id, sha256, filename, subject, from_addr, to_addrs, cc_addrs,
		message_id, COALESCE(thread_id, '') AS thread_id, sent_at, received_at, size_bytes,
		has_attachments, attachment_count, imported_at, starred, category_id`
	// Rows without a thread key group with themselves, so singletons still list.
	const gkey = `COALESCE(thread_id, 'solo:' || id)`

	countQ := "SELECT COUNT(*) FROM emails " + where
	listQ := `SELECT ` + cols + ` FROM emails ` + where +
		` ORDER BY ` + sortCol + ` ` + order + ` LIMIT ? OFFSET ?`
	if opts.GroupThreads {
		countQ = "SELECT COUNT(DISTINCT " + gkey + ") FROM emails " + where
		listQ = `SELECT id, sha256, filename, subject, from_addr, to_addrs, cc_addrs,
			message_id, thread_id, sent_at, received_at, size_bytes,
			has_attachments, attachment_count, imported_at, starred, category_id, cnt FROM (
			SELECT ` + cols + `,
				COUNT(*) OVER (PARTITION BY ` + gkey + `) AS cnt,
				ROW_NUMBER() OVER (PARTITION BY ` + gkey + ` ORDER BY sent_at DESC, id DESC) AS rn
			FROM emails ` + where + `
		) WHERE rn = 1 ORDER BY ` + sortCol + ` ` + order + ` LIMIT ? OFFSET ?`
	}

	var total int
	if err := s.DB.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, opts.Limit, opts.Offset)
	rows, err := s.DB.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Email{}
	for rows.Next() {
		var cnt int
		var sc rowScanner = rows
		if opts.GroupThreads {
			sc = extraScanner{rows: rows, extra: []any{&cnt}}
		}
		e, err := scanEmail(sc)
		if err != nil {
			return nil, 0, err
		}
		e.ThreadCount = cnt
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// extraScanner appends fixed destinations to every Scan, so scanEmail's column
// list can be reused by queries that select trailing extras.
type extraScanner struct {
	rows  *sql.Rows
	extra []any
}

func (s extraScanner) Scan(dest ...any) error {
	return s.rows.Scan(append(dest, s.extra...)...)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEmail(rs rowScanner) (*Email, error) {
	e := Email{}
	var to, cc string
	var sentAt, recvAt, importedAt int64
	var hasAtt, starred int
	var categoryID sql.NullInt64
	err := rs.Scan(&e.ID, &e.SHA256, &e.Filename, &e.Subject, &e.FromAddr,
		&to, &cc, &e.MessageID, &e.ThreadID, &sentAt, &recvAt, &e.SizeBytes,
		&hasAtt, &e.AttachmentCount, &importedAt, &starred, &categoryID)
	if err != nil {
		return nil, err
	}
	e.Starred = starred != 0
	if categoryID.Valid {
		id := categoryID.Int64
		e.CategoryID = &id
	}
	if to != "" {
		_ = json.Unmarshal([]byte(to), &e.ToAddrs)
	}
	if cc != "" {
		_ = json.Unmarshal([]byte(cc), &e.CcAddrs)
	}
	if sentAt > 0 {
		e.SentAt = time.Unix(sentAt, 0).UTC()
	}
	if recvAt > 0 {
		e.ReceivedAt = time.Unix(recvAt, 0).UTC()
	}
	if importedAt > 0 {
		e.ImportedAt = time.Unix(importedAt, 0).UTC()
	}
	e.HasAttachments = hasAtt != 0
	return &e, nil
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

// nullIfEmpty keeps "no thread" as NULL so the partial index stays small and
// the backfill's IS NULL filter means "never derived", not "derived as none".
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func stringsOrEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "unique constraint")
}

func buildFTSQuery(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	terms := strings.Fields(raw)
	parts := make([]string, 0, len(terms))
	for _, t := range terms {
		t = strings.ReplaceAll(t, `"`, `""`)
		parts = append(parts, `"`+t+`"*`)
	}
	return strings.Join(parts, " ")
}
