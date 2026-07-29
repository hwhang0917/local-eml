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

const threadRootEML = `From: alice@example.com
To: bob@example.com
Subject: kickoff
Date: Wed, 20 May 2026 09:00:00 +0900
Message-ID: <root@example.com>

let's begin
`

const threadReplyEML = `From: bob@example.com
To: alice@example.com
Subject: Re: kickoff
Date: Wed, 20 May 2026 10:00:00 +0900
Message-ID: <reply@example.com>
In-Reply-To: <root@example.com>
References: <root@example.com>

sounds good
`

func TestImportDerivesThreadID(t *testing.T) {
	ctx := context.Background()
	p := newTestPaths(t)
	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	im := &Importer{Store: st, Paths: p}

	dir := t.TempDir()
	for name, body := range map[string]string{
		"root.eml": threadRootEML, "reply.eml": threadReplyEML,
	} {
		if _, err := im.ImportFile(ctx, writeEML(t, dir, name, body), name); err != nil {
			t.Fatal(err)
		}
	}
	emails, _, err := st.ListEmails(ctx, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range emails {
		if e.ThreadID != "root@example.com" {
			t.Errorf("%s thread_id = %q, want root@example.com", e.Subject, e.ThreadID)
		}
	}
	thread, err := st.ListThread(ctx, "root@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 2 || thread[0].Subject != "kickoff" {
		t.Fatalf("thread order wrong: %+v", thread)
	}
}

func TestBackfillThreadIDs(t *testing.T) {
	ctx := context.Background()
	p := newTestPaths(t)
	st, err := store.Open(ctx, p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	im := &Importer{Store: st, Paths: p}

	dir := t.TempDir()
	for name, body := range map[string]string{
		"root.eml": threadRootEML, "reply.eml": threadReplyEML,
	} {
		if _, err := im.ImportFile(ctx, writeEML(t, dir, name, body), name); err != nil {
			t.Fatal(err)
		}
	}
	// Simulate rows written before thread_id existed.
	if _, err := st.DB.ExecContext(ctx, `UPDATE emails SET thread_id = NULL`); err != nil {
		t.Fatal(err)
	}
	if err := st.SetUserVersion(ctx, 1); err != nil {
		t.Fatal(err)
	}

	n, err := im.BackfillThreadIDs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 repaired rows, got %d", n)
	}
	thread, err := st.ListThread(ctx, "root@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 2 {
		t.Fatalf("thread has %d members after backfill, want 2", len(thread))
	}

	// The marker must stop a second pass.
	if _, err := st.DB.ExecContext(ctx, `UPDATE emails SET thread_id = NULL`); err != nil {
		t.Fatal(err)
	}
	if n, err = im.BackfillThreadIDs(ctx); err != nil || n != 0 {
		t.Fatalf("second run should be a no-op, got n=%d err=%v", n, err)
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
