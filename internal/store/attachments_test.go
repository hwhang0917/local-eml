package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

// seedAttachments stores three messages:
//
//	a (2024-01-10, Alice): invoice.pdf 300 B, photo.JPG 2000 B
//	b (2025-06-01, Bob):   notes.hwp 50 B, data.bin 10 B (octet-stream)
//	c (2025-07-01, Mallory, flagged spam): payload.zip 999 B
func seedAttachments(t *testing.T, st *Store) {
	t.Helper()
	ctx := context.Background()
	day := func(s string) time.Time {
		d, _ := time.Parse(time.DateOnly, s)
		return d
	}
	rows := []EmailRow{
		{
			Email: Email{SHA256: strings.Repeat("a", 64), Subject: "Invoice", FromAddr: "Alice <alice@example.com>",
				SentAt: day("2024-01-10"), AttachmentCount: 2, HasAttachments: true},
			Attachments: []AttachmentMeta{
				{Index: 0, Filename: "invoice.pdf", ContentType: "application/pdf", Size: 300},
				{Index: 1, Filename: "photo.JPG", ContentType: "application/octet-stream", Size: 2000},
			},
		},
		{
			Email: Email{SHA256: strings.Repeat("b", 64), Subject: "Notes", FromAddr: "bob@example.com",
				SentAt: day("2025-06-01"), AttachmentCount: 2, HasAttachments: true},
			Attachments: []AttachmentMeta{
				{Index: 0, Filename: "notes.hwp", ContentType: "application/octet-stream", Size: 50},
				{Index: 1, Filename: "data.bin", ContentType: "Application/Octet-Stream", Size: 10},
			},
		},
		{
			Email: Email{SHA256: strings.Repeat("c", 64), Subject: "Prize", FromAddr: "mallory@example.com",
				SentAt: day("2025-07-01"), AttachmentCount: 1, HasAttachments: true},
			Attachments: []AttachmentMeta{
				{Index: 0, Filename: "payload.zip", ContentType: "application/zip", Size: 999},
			},
		},
	}
	for _, r := range rows {
		if _, err := st.InsertEmail(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.SetEmailFlag(ctx, strings.Repeat("c", 64), FlagSpam); err != nil {
		t.Fatal(err)
	}
}

func names(l *AttachmentList) []string {
	out := []string{}
	for _, a := range l.Items {
		out = append(out, a.Filename)
	}
	return out
}

func TestListAttachments(t *testing.T) {
	st := openTestStore(t)
	seedAttachments(t, st)
	ctx := context.Background()
	list := func(o AttachmentListOptions) *AttachmentList {
		t.Helper()
		l, err := st.ListAttachments(ctx, o)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}

	all := list(AttachmentListOptions{})
	// Default order is newest message first, then part order; quarantined
	// mail is left out entirely.
	if got := strings.Join(names(all), ","); got != "notes.hwp,data.bin,invoice.pdf,photo.JPG" {
		t.Errorf("default order = %s", got)
	}
	if all.Total != 4 || all.TotalBytes != 2360 {
		t.Errorf("totals = %d / %d, want 4 / 2360", all.Total, all.TotalBytes)
	}
	wantKinds := map[string]int{KindPDF: 1, KindImage: 1, KindDocument: 1, KindOther: 1}
	for _, k := range AttachmentKinds {
		if all.Kinds[k] != wantKinds[k] {
			t.Errorf("kinds[%s] = %d, want %d", k, all.Kinds[k], wantKinds[k])
		}
	}
	a := all.Items[2]
	if a.EmailSHA256 != strings.Repeat("a", 64) || a.EmailSubject != "Invoice" || a.Index != 0 || a.Kind != KindPDF {
		t.Errorf("row = %+v", a)
	}
	if all.Items[1].ContentType != "application/octet-stream" {
		t.Errorf("content type not lowercased: %q", all.Items[1].ContentType)
	}

	if got := strings.Join(names(list(AttachmentListOptions{Sort: "size", Order: "desc"})), ","); got != "photo.JPG,invoice.pdf,notes.hwp,data.bin" {
		t.Errorf("size desc = %s", got)
	}
	if got := strings.Join(names(list(AttachmentListOptions{Sort: "name", Order: "asc"})), ","); got != "data.bin,invoice.pdf,notes.hwp,photo.JPG" {
		t.Errorf("name asc = %s", got)
	}

	img := list(AttachmentListOptions{Kind: KindImage})
	if got := strings.Join(names(img), ","); got != "photo.JPG" || img.Total != 1 || img.TotalBytes != 2000 {
		t.Errorf("image filter = %s (%d, %d)", got, img.Total, img.TotalBytes)
	}
	// The kind counts ignore the kind filter so the picker can show them all.
	if img.Kinds[KindPDF] != 1 {
		t.Errorf("kind counts narrowed by kind filter: %v", img.Kinds)
	}

	if got := strings.Join(names(list(AttachmentListOptions{Name: "INVOICE"})), ","); got != "invoice.pdf" {
		t.Errorf("name filter = %s", got)
	}
	// LIKE wildcards in the query are literal.
	if got := list(AttachmentListOptions{Name: "%"}); got.Total != 0 {
		t.Errorf("%% matched %d rows", got.Total)
	}
	if got := strings.Join(names(list(AttachmentListOptions{Sender: "alice"})), ","); got != "invoice.pdf,photo.JPG" {
		t.Errorf("sender filter = %s", got)
	}
	from, _ := time.Parse(time.DateOnly, "2025-01-01")
	if got := list(AttachmentListOptions{From: from.Unix()}); got.Total != 2 {
		t.Errorf("date filter total = %d, want 2", got.Total)
	}

	page := list(AttachmentListOptions{Limit: 10, Offset: 3})
	if len(page.Items) != 1 || page.Total != 4 {
		t.Errorf("offset page = %d items of %d", len(page.Items), page.Total)
	}
}

func TestAttachmentsGoWithTheirEmail(t *testing.T) {
	st := openTestStore(t)
	seedAttachments(t, st)
	ctx := context.Background()
	if err := st.DeleteEmailBySHA(ctx, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM attachments`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("attachments left = %d, want 3", n)
	}
}

func TestAttachmentKindMatchesSQL(t *testing.T) {
	st := openTestStore(t)
	cases := []struct{ ct, name, want string }{
		{"application/pdf", "x", KindPDF},
		{"application/octet-stream", "scan.PDF", KindPDF},
		{"image/svg+xml", "logo.svg", KindImage},
		{"application/octet-stream", "IMG_0001.HEIC", KindImage},
		{"video/mp4", "", KindMedia},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "q.xlsx", KindSpreadsheet},
		{"text/csv", "", KindSpreadsheet},
		{"application/x-hwp", "", KindDocument},
		{"text/plain", "readme", KindDocument},
		{"application/x-7z-compressed", "", KindArchive},
		{"application/octet-stream", "backup.tar.gz", KindArchive},
		{"application/octet-stream", "setup.exe", KindOther},
		{"", "", KindOther},
	}
	for _, c := range cases {
		if got := AttachmentKind(c.ct, c.name); got != c.want {
			t.Errorf("AttachmentKind(%q, %q) = %s, want %s", c.ct, c.name, got, c.want)
		}
		var sqlKind string
		err := st.DB.QueryRow(`SELECT `+kindExpr+` FROM (SELECT lower(?) AS content_type, ? AS filename) a`,
			c.ct, c.name).Scan(&sqlKind)
		if err != nil {
			t.Fatal(err)
		}
		if sqlKind != c.want {
			t.Errorf("kindExpr(%q, %q) = %s, want %s", c.ct, c.name, sqlKind, c.want)
		}
	}
}
