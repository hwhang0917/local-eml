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
	SentAt          time.Time `json:"sent_at"`
	ReceivedAt      time.Time `json:"received_at"`
	SizeBytes       int64     `json:"size_bytes"`
	HasAttachments  bool      `json:"has_attachments"`
	AttachmentCount int       `json:"attachment_count"`
	ImportedAt      time.Time `json:"imported_at"`
	Starred         bool      `json:"starred"`
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

	chosung := ToChosung(e.Subject + " " + e.FromAddr)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO emails (sha256, filename, subject, from_addr, to_addrs, cc_addrs,
			message_id, sent_at, received_at, size_bytes, has_attachments,
			attachment_count, imported_at, chosung_text)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		e.SHA256, e.Filename, e.Subject, e.FromAddr, string(to), string(cc),
		e.MessageID, unixOrZero(e.SentAt), unixOrZero(e.ReceivedAt), e.SizeBytes,
		hasAtt, e.AttachmentCount, time.Now().Unix(), chosung,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicate
		}
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
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
			message_id, sent_at, received_at, size_bytes, has_attachments,
			attachment_count, imported_at, starred
		FROM emails WHERE sha256 = ?`, sha)
	return scanEmail(row)
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
	Query        string
	StarredOnly  bool
	Sort         string
	Order        string
	Limit        int
	Offset       int
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
	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM emails "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQ := `SELECT id, sha256, filename, subject, from_addr, to_addrs, cc_addrs,
		message_id, sent_at, received_at, size_bytes, has_attachments,
		attachment_count, imported_at, starred FROM emails ` + where +
		` ORDER BY ` + sortCol + ` ` + order + ` LIMIT ? OFFSET ?`
	args = append(args, opts.Limit, opts.Offset)
	rows, err := s.DB.QueryContext(ctx, listQ, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Email{}
	for rows.Next() {
		e, err := scanEmail(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEmail(rs rowScanner) (*Email, error) {
	e := Email{}
	var to, cc string
	var sentAt, recvAt, importedAt int64
	var hasAtt, starred int
	err := rs.Scan(&e.ID, &e.SHA256, &e.Filename, &e.Subject, &e.FromAddr,
		&to, &cc, &e.MessageID, &sentAt, &recvAt, &e.SizeBytes,
		&hasAtt, &e.AttachmentCount, &importedAt, &starred)
	if err != nil {
		return nil, err
	}
	e.Starred = starred != 0
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
