package importer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

// An HTML body with one embedded cid: image and no real attachment.
const inlineOnlyEML = `From: alice@example.com
To: bob@example.com
Subject: Inline only
Date: Wed, 20 May 2026 11:30:00 +0900
MIME-Version: 1.0
Content-Type: multipart/related; boundary="REL"

--REL
Content-Type: text/html; charset=utf-8

<p>logo: <img src="cid:logo@example"></p>
--REL
Content-Type: image/png; name="logo.png"
Content-ID: <logo@example>
Content-Disposition: inline; filename="logo.png"
Content-Transfer-Encoding: base64

iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==
--REL--
`

func writeEML(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInlineImagesAreNotAttachments(t *testing.T) {
	ctx := context.Background()
	p := newTestPaths(t)
	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	im := &Importer{Store: st, Paths: p}

	src := writeEML(t, t.TempDir(), "inline.eml", inlineOnlyEML)
	res, err := im.ImportFile(ctx, src, "inline.eml")
	if err != nil {
		t.Fatal(err)
	}
	e, err := st.GetEmailBySHA(ctx, res.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if e.HasAttachments || e.AttachmentCount != 0 {
		t.Fatalf("inline-only email counted as attachment: has=%v count=%d",
			e.HasAttachments, e.AttachmentCount)
	}
}

func TestBackfillAttachmentCounts(t *testing.T) {
	ctx := context.Background()
	p := newTestPaths(t)
	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	im := &Importer{Store: st, Paths: p}

	src := writeEML(t, t.TempDir(), "inline.eml", inlineOnlyEML)
	res, err := im.ImportFile(ctx, src, "inline.eml")
	if err != nil {
		t.Fatal(err)
	}

	// Simulate a row written by the old parser, which counted the inline image.
	if _, err := st.DB.ExecContext(ctx,
		`UPDATE emails SET has_attachments = 1, attachment_count = 1 WHERE sha256 = ?`,
		res.SHA256); err != nil {
		t.Fatal(err)
	}

	n, err := im.BackfillAttachmentCounts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expected 1 repaired row, got %d", n)
	}
	e, err := st.GetEmailBySHA(ctx, res.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if e.HasAttachments || e.AttachmentCount != 0 {
		t.Fatalf("backfill left stale values: has=%v count=%d", e.HasAttachments, e.AttachmentCount)
	}

	// The marker must stop a second pass even if values drift again.
	if _, err := st.DB.ExecContext(ctx,
		`UPDATE emails SET attachment_count = 9 WHERE sha256 = ?`, res.SHA256); err != nil {
		t.Fatal(err)
	}
	if n, err = im.BackfillAttachmentCounts(ctx); err != nil || n != 0 {
		t.Fatalf("second run should be a no-op, got n=%d err=%v", n, err)
	}
}
