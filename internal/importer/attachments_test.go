package importer

import (
	"context"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

// A plain-text body, one inline cid: image (not an attachment) and two real
// attachments, one with a Korean RFC 2047 filename.
const twoAttachmentsEML = `From: alice@example.com
To: bob@example.com
Subject: Contract
Date: Wed, 20 May 2026 11:30:00 +0900
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary="MIX"

--MIX
Content-Type: multipart/related; boundary="REL"

--REL
Content-Type: text/html; charset=utf-8

<p>see attached <img src="cid:logo@example"></p>
--REL
Content-Type: image/png; name="logo.png"
Content-ID: <logo@example>
Content-Disposition: inline; filename="logo.png"
Content-Transfer-Encoding: base64

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==
--REL--
--MIX
Content-Type: application/pdf
Content-Disposition: attachment; filename="=?UTF-8?B?6rOE7JW97IScLnBkZg==?="

%PDF-1.4 fake
--MIX
Content-Type: application/octet-stream
Content-Disposition: attachment; filename="photo.jpg"

not really a jpeg
--MIX--
`

func attachmentRows(t *testing.T, st *store.Store) []store.Attachment {
	t.Helper()
	l, err := st.ListAttachments(context.Background(), store.AttachmentListOptions{Sort: "name", Order: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	return l.Items
}

func checkContractAttachments(t *testing.T, got []store.Attachment) {
	t.Helper()
	if len(got) != 2 {
		t.Fatalf("attachments = %+v, want 2", got)
	}
	// Sorted by name: "photo.jpg" < "계약서.pdf".
	if got[0].Filename != "photo.jpg" || got[0].Index != 1 || got[0].Kind != store.KindImage {
		t.Errorf("first = %+v", got[0])
	}
	if got[1].Filename != "계약서.pdf" || got[1].Index != 0 || got[1].Kind != store.KindPDF ||
		got[1].ContentType != "application/pdf" || got[1].Size != int64(len("%PDF-1.4 fake")) {
		t.Errorf("second = %+v", got[1])
	}
}

func TestImportRecordsAttachments(t *testing.T) {
	ctx := context.Background()
	p := newTestPaths(t)
	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	im := &Importer{Store: st, Paths: p}

	if _, err := im.ImportFile(ctx, writeEML(t, t.TempDir(), "c.eml", twoAttachmentsEML), "c.eml"); err != nil {
		t.Fatal(err)
	}
	checkContractAttachments(t, attachmentRows(t, st))
}

func TestBackfillAttachments(t *testing.T) {
	ctx := context.Background()
	p := newTestPaths(t)
	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	im := &Importer{Store: st, Paths: p}

	if _, err := im.ImportFile(ctx, writeEML(t, t.TempDir(), "c.eml", twoAttachmentsEML), "c.eml"); err != nil {
		t.Fatal(err)
	}
	// Simulate a database from before the table existed.
	if _, err := st.DB.ExecContext(ctx, `DELETE FROM attachments`); err != nil {
		t.Fatal(err)
	}
	n, err := im.BackfillAttachments(ctx)
	if err != nil || n != 1 {
		t.Fatalf("backfill = %d, %v; want 1", n, err)
	}
	checkContractAttachments(t, attachmentRows(t, st))

	// Nothing left to do on the next start.
	if n, err := im.BackfillAttachments(ctx); err != nil || n != 0 {
		t.Fatalf("second backfill = %d, %v; want 0", n, err)
	}
}
