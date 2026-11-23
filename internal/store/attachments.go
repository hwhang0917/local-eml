package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AttachmentMeta is what the attachments table keeps about one part. Index is
// the part's position among the parsed message's attachments.
type AttachmentMeta struct {
	Index       int
	Filename    string
	ContentType string
	Size        int64
}

// Attachment is one row of the attachment browser: the part plus enough of
// its message to sort, filter and link back to it.
type Attachment struct {
	EmailSHA256  string `json:"email_sha256"`
	EmailSubject string `json:"email_subject"`
	EmailFrom    string `json:"email_from"`
	// SentAt is the message's date; zero when it had no parseable Date header.
	SentAt      int64  `json:"sent_at"`
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Kind        string `json:"kind"`
}

// Attachment kinds: a coarse grouping for the type filter. A part is one kind
// only; the first match in kindRules order wins, and anything unmatched is
// KindOther.
const (
	KindImage       = "image"
	KindPDF         = "pdf"
	KindDocument    = "document"
	KindSpreadsheet = "spreadsheet"
	KindArchive     = "archive"
	KindMedia       = "media"
	KindOther       = "other"
)

// AttachmentKinds lists every kind in display order.
var AttachmentKinds = []string{
	KindImage, KindPDF, KindDocument, KindSpreadsheet, KindArchive, KindMedia, KindOther,
}

// kindRules classify by declared type first and file extension second: plenty
// of senders label everything application/octet-stream, and the extension is
// then the only clue. Kept as data so the SQL below and the tests agree.
var kindRules = []struct {
	kind     string
	prefixes []string // content_type prefixes
	types    []string // exact content types
	exts     []string // lowercase extensions without the dot
}{
	{kind: KindPDF, types: []string{"application/pdf"}, exts: []string{"pdf"}},
	{
		kind:     KindImage,
		prefixes: []string{"image/"},
		exts:     []string{"png", "jpg", "jpeg", "gif", "webp", "bmp", "heic", "heif", "tif", "tiff", "svg"},
	},
	{
		kind:     KindMedia,
		prefixes: []string{"audio/", "video/"},
		exts:     []string{"mp3", "m4a", "wav", "ogg", "flac", "aac", "mp4", "mov", "m4v", "avi", "mkv", "webm"},
	},
	{
		kind: KindSpreadsheet,
		types: []string{
			"application/vnd.ms-excel",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			"application/vnd.oasis.opendocument.spreadsheet",
			"text/csv",
		},
		exts: []string{"xls", "xlsx", "xlsm", "csv", "ods", "numbers", "cell"},
	},
	{
		kind: KindDocument,
		types: []string{
			"application/msword",
			"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			"application/vnd.ms-powerpoint",
			"application/vnd.openxmlformats-officedocument.presentationml.presentation",
			"application/vnd.oasis.opendocument.text",
			"application/vnd.oasis.opendocument.presentation",
			"application/rtf",
			"application/x-hwp",
			"application/haansofthwp",
			"application/vnd.hancom.hwp",
			"application/vnd.hancom.hwpx",
			"text/plain",
			"text/markdown",
		},
		exts: []string{
			"doc", "docx", "odt", "rtf", "txt", "md", "pages",
			"ppt", "pptx", "odp", "key", "show",
			"hwp", "hwpx",
		},
	},
	{
		kind: KindArchive,
		types: []string{
			"application/zip",
			"application/x-zip-compressed",
			"application/x-7z-compressed",
			"application/x-rar-compressed",
			"application/vnd.rar",
			"application/gzip",
			"application/x-gzip",
			"application/x-tar",
		},
		exts: []string{"zip", "7z", "rar", "gz", "tgz", "tar", "bz2", "xz", "alz", "egg"},
	},
}

// kindExpr is a SQL CASE that classifies an attachments row (aliased a) the
// same way AttachmentKind does. Computed at query time rather than stored, so
// refining the rules needs no backfill.
var kindExpr = func() string {
	var b strings.Builder
	b.WriteString("CASE")
	for _, r := range kindRules {
		conds := []string{}
		for _, p := range r.prefixes {
			conds = append(conds, fmt.Sprintf("a.content_type LIKE '%s%%'", p))
		}
		if len(r.types) > 0 {
			conds = append(conds, "a.content_type IN ('"+strings.Join(r.types, "','")+"')")
		}
		for _, e := range r.exts {
			conds = append(conds, fmt.Sprintf("lower(a.filename) LIKE '%%.%s'", e))
		}
		fmt.Fprintf(&b, " WHEN %s THEN '%s'", strings.Join(conds, " OR "), r.kind)
	}
	fmt.Fprintf(&b, " ELSE '%s' END", KindOther)
	return b.String()
}()

// AttachmentKind classifies one part; the Go twin of kindExpr.
func AttachmentKind(contentType, filename string) string {
	ct := strings.ToLower(contentType)
	name := strings.ToLower(filename)
	for _, r := range kindRules {
		for _, p := range r.prefixes {
			if strings.HasPrefix(ct, p) {
				return r.kind
			}
		}
		for _, t := range r.types {
			if ct == t {
				return r.kind
			}
		}
		for _, e := range r.exts {
			if strings.HasSuffix(name, "."+e) {
				return r.kind
			}
		}
	}
	return KindOther
}

func insertAttachments(ctx context.Context, tx *sql.Tx, emailID int64, atts []AttachmentMeta) error {
	if len(atts) == 0 {
		return nil
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO attachments (email_id, idx, filename, content_type, size_bytes)
		VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, a := range atts {
		if _, err := stmt.ExecContext(ctx, emailID, a.Index, a.Filename,
			strings.ToLower(a.ContentType), a.Size); err != nil {
			return err
		}
	}
	return nil
}

// SetEmailAttachments records the attachments of an existing row, for the
// backfill. Rows already there are replaced, so a re-run is harmless.
func (s *Store) SetEmailAttachments(ctx context.Context, emailID int64, atts []AttachmentMeta) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := insertAttachments(ctx, tx, emailID, atts); err != nil {
		return err
	}
	return tx.Commit()
}

// ListEmailsMissingAttachments returns rows that claim attachments but have
// none recorded: everything imported before the table existed.
func (s *Store) ListEmailsMissingAttachments(ctx context.Context) ([]BackfillRow, error) {
	return s.listBackfillRows(ctx, `attachment_count > 0
		AND NOT EXISTS (SELECT 1 FROM attachments a WHERE a.email_id = emails.id)`)
}

// AttachmentListOptions filters and orders ListAttachments.
type AttachmentListOptions struct {
	// Name matches anywhere in the filename, case-insensitively for ASCII.
	Name string
	// Kind is one of AttachmentKinds, or "" for all.
	Kind string
	// Sender matches anywhere in the message's From header.
	Sender string
	// From and To bound the message's sent_at (unix seconds); 0 is unbounded.
	From, To int64
	Sort     string // name | type | size | date | sender
	Order    string // asc | desc
	Limit    int
	Offset   int
}

// AttachmentList is one page of attachments plus totals over every match.
type AttachmentList struct {
	Items      []Attachment `json:"items"`
	Total      int          `json:"total"`
	TotalBytes int64        `json:"total_bytes"`
	// Kinds counts matches per kind ignoring the Kind filter, so the type
	// picker can show what each choice would give.
	Kinds map[string]int `json:"kinds"`
}

// attachmentSortCols name columns of the listing subquery in ListAttachments.
var attachmentSortCols = map[string]string{
	"name":   "filename COLLATE NOCASE",
	"type":   "kind",
	"size":   "size_bytes",
	"date":   "sent_at",
	"sender": "sender COLLATE NOCASE",
}

// ListAttachments pages through the attachments of every visible message.
// Quarantined mail is left out: the server refuses its attachments anyway.
func (s *Store) ListAttachments(ctx context.Context, opts AttachmentListOptions) (*AttachmentList, error) {
	opts.Limit = ClampPageSize(opts.Limit)
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	sortCol, ok := attachmentSortCols[opts.Sort]
	if !ok {
		sortCol = attachmentSortCols["date"]
	}
	order := "DESC"
	if strings.EqualFold(opts.Order, "asc") {
		order = "ASC"
	}

	conds := []string{"e." + notQuarantined}
	args := []any{}
	if n := strings.TrimSpace(opts.Name); n != "" {
		conds = append(conds, `a.filename LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(n)+"%")
	}
	if snd := strings.TrimSpace(opts.Sender); snd != "" {
		conds = append(conds, `e.from_addr LIKE ? ESCAPE '\'`)
		args = append(args, "%"+escapeLike(snd)+"%")
	}
	if opts.From > 0 {
		conds = append(conds, `e.sent_at >= ?`)
		args = append(args, opts.From)
	}
	if opts.To > 0 {
		conds = append(conds, `e.sent_at <= ?`)
		args = append(args, opts.To)
	}
	base := `FROM attachments a JOIN emails e ON e.id = a.email_id WHERE ` + strings.Join(conds, " AND ")

	out := &AttachmentList{Items: []Attachment{}, Kinds: map[string]int{}}
	for _, k := range AttachmentKinds {
		out.Kinds[k] = 0
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT kind, COUNT(*), SUM(size) FROM (SELECT `+kindExpr+` AS kind, a.size_bytes AS size `+base+`)
		 GROUP BY kind`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var kind string
		var n int
		var bytes int64
		if err := rows.Scan(&kind, &n, &bytes); err != nil {
			rows.Close()
			return nil, err
		}
		out.Kinds[kind] = n
		if opts.Kind == "" || opts.Kind == kind {
			out.Total += n
			out.TotalBytes += bytes
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	listQ := `SELECT sha256, subject, sender, sent_at, idx, filename, content_type, size_bytes, kind
		FROM (SELECT e.sha256, COALESCE(e.subject, '') AS subject, COALESCE(e.from_addr, '') AS sender,
			COALESCE(e.sent_at, 0) AS sent_at, a.email_id, a.idx, a.filename, a.content_type, a.size_bytes,
			` + kindExpr + ` AS kind ` + base + `)`
	listArgs := append([]any{}, args...)
	if opts.Kind != "" {
		listQ += ` WHERE kind = ?`
		listArgs = append(listArgs, opts.Kind)
	}
	// Ties fall back to newest message, then part order, so paging is stable.
	listQ += ` ORDER BY ` + sortCol + ` ` + order + `, sent_at DESC, email_id DESC, idx ASC LIMIT ? OFFSET ?`
	listArgs = append(listArgs, opts.Limit, opts.Offset)

	rows, err = s.DB.QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.EmailSHA256, &a.EmailSubject, &a.EmailFrom, &a.SentAt, &a.Index,
			&a.Filename, &a.ContentType, &a.Size, &a.Kind); err != nil {
			return nil, err
		}
		out.Items = append(out.Items, a)
	}
	return out, rows.Err()
}

// escapeLike makes s match literally inside a LIKE pattern with ESCAPE '\'.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
