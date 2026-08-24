package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRestoreMetadata_MergesStarsProfilesAndScrubsPasswords(t *testing.T) {
	ctx := context.Background()
	src := openTestStore(t)

	sha := "1111111111111111111111111111111111111111111111111111111111111111"
	row := EmailRow{Email: Email{SHA256: sha, Filename: "a.eml", Subject: "s", FromAddr: "f@example"}, BodyText: "b"}
	if _, err := src.InsertEmail(ctx, row); err != nil {
		t.Fatal(err)
	}
	if _, err := src.DB.ExecContext(ctx, `UPDATE emails SET starred = 1 WHERE sha256 = ?`, sha); err != nil {
		t.Fatal(err)
	}
	if _, err := src.DB.ExecContext(ctx,
		`INSERT INTO imap_profiles (name, host, username, created_at, updated_at, sync_enabled, encrypted_password)
		 VALUES ('work', 'imap.example.com', 'u', 1, 1, 1, x'deadbeef')`); err != nil {
		t.Fatal(err)
	}

	snap := filepath.Join(t.TempDir(), "snap.db")
	if err := src.SnapshotTo(ctx, snap); err != nil {
		t.Fatal(err)
	}

	// Fresh install: same email imported (blob re-import), no star, no profiles.
	dst := openTestStore(t)
	if _, err := dst.InsertEmail(ctx, row); err != nil {
		t.Fatal(err)
	}

	sum, err := dst.RestoreMetadata(ctx, snap)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Emails != 1 || sum.ImapProfiles != 1 {
		t.Fatalf("summary=%+v, want 1 email and 1 imap profile", sum)
	}

	var starred int
	if err := dst.DB.QueryRowContext(ctx,
		`SELECT starred FROM emails WHERE sha256 = ?`, sha).Scan(&starred); err != nil {
		t.Fatal(err)
	}
	if starred != 1 {
		t.Errorf("starred=%d, want 1 restored", starred)
	}

	var syncEnabled int
	var pw []byte
	if err := dst.DB.QueryRowContext(ctx,
		`SELECT sync_enabled, encrypted_password FROM imap_profiles WHERE name = 'work'`).
		Scan(&syncEnabled, &pw); err != nil {
		t.Fatal(err)
	}
	if syncEnabled != 0 || pw != nil {
		t.Errorf("sync_enabled=%d pw=%v, want password scrubbed and sync off", syncEnabled, pw)
	}

	// Idempotent: a second run must not duplicate profiles.
	if _, err := dst.RestoreMetadata(ctx, snap); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := dst.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM imap_profiles`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("imap_profiles=%d after re-restore, want 1", n)
	}
}
