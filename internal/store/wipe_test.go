package store

import (
	"context"
	"strings"
	"testing"
)

func TestWipeLibrary(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.InsertEmail(ctx, EmailRow{
		Email:    Email{SHA256: strings.Repeat("a", 64), Subject: "quarterly report"},
		BodyText: "confidential numbers",
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateImport(ctx, Import{ID: "imp1", SourceKind: "imap", SourceName: "imap://me@host/INBOX", Status: "done"}); err != nil {
		t.Fatal(err)
	}
	p, err := st.UpsertIMAPProfile(ctx, IMAPProfile{Name: "work", Host: "imap.example.com", Username: "me", SyncEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetIMAPProfilePassword(ctx, p.ID, []byte("ciphertext")); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateIMAPProfileSyncState(ctx, p.ID, 7, 42, 1); err != nil {
		t.Fatal(err)
	}

	n, err := st.WipeLibrary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("WipeLibrary reported %d emails, want 1", n)
	}

	if _, total, err := st.ListEmails(ctx, ListOptions{Limit: 10}); err != nil || total != 0 {
		t.Fatalf("emails left: total=%d err=%v", total, err)
	}
	var hits int
	if err := st.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM emails_fts WHERE emails_fts MATCH 'confidential'`).Scan(&hits); err != nil || hits != 0 {
		t.Fatalf("search index still matches wiped mail: hits=%d err=%v", hits, err)
	}
	var imports int
	if err := st.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM imports`).Scan(&imports); err != nil || imports != 0 {
		t.Fatalf("import history left: %d, %v", imports, err)
	}

	// The profile stays, but it must not refill the library on the next tick.
	got, err := st.GetIMAPProfile(ctx, p.ID)
	if err != nil {
		t.Fatalf("profile must survive a wipe: %v", err)
	}
	if got.SyncEnabled || got.HasPassword || got.LastUID != nil || got.UIDValidity != nil {
		t.Fatalf("profile still set up to sync: %+v", got)
	}
}
