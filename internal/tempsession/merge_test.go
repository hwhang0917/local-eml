package tempsession

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

func testEML(n int) string {
	return fmt.Sprintf("From: a@example.com\r\nTo: b@example.com\r\nSubject: mail %d\r\n"+
		"Message-ID: <m%d@example.com>\r\nDate: Mon, 2 Jan 2006 15:04:05 +0000\r\n\r\nbody %d\r\n", n, n, n)
}

// library opens a fresh library at base, the way run does.
func library(t *testing.T, base string) (*importer.Importer, paths.Paths) {
	t.Helper()
	p := paths.At(base)
	if err := p.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return &importer.Importer{Store: st, Paths: p}, p
}

func importEML(t *testing.T, im *importer.Importer, n int) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "in.eml")
	if err := os.WriteFile(f, []byte(testEML(n)), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := im.ImportFile(context.Background(), f, fmt.Sprintf("mail-%d.eml", n))
	if err != nil {
		t.Fatal(err)
	}
	return res.SHA256
}

func TestMergeKeepsSessionMailWithoutClobberingLibrary(t *testing.T) {
	ctx := context.Background()
	sessDir, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := library(t, sessDir)
	lib, _ := library(t, t.TempDir())

	// Mail 1 is in both: read in the library, unread in the session.
	shared := importEML(t, lib, 1)
	if err := lib.Store.SetEmailRead(ctx, shared, true); err != nil {
		t.Fatal(err)
	}
	cats, _ := lib.Store.ListCategories(ctx)
	if _, err := lib.Store.RenameCategory(ctx, cats[0].ID, "Mine"); err != nil {
		t.Fatal(err)
	}
	if err := lib.Store.SetSetting(ctx, "read_tracking", "on"); err != nil {
		t.Fatal(err)
	}

	importEML(t, sess, 1)
	fresh := importEML(t, sess, 2)
	if err := sess.Store.SetEmailStarred(ctx, fresh, true); err != nil {
		t.Fatal(err)
	}
	if err := sess.Store.SetEmailRead(ctx, fresh, true); err != nil {
		t.Fatal(err)
	}
	scats, _ := sess.Store.ListCategories(ctx)
	if err := sess.Store.SetEmailCategory(ctx, fresh, &scats[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := sess.Store.SetSetting(ctx, "read_tracking", "off"); err != nil {
		t.Fatal(err)
	}
	if err := sess.Store.SetSetting(ctx, "only_in_session", "x"); err != nil {
		t.Fatal(err)
	}
	prof, err := sess.Store.UpsertIMAPProfile(ctx, store.IMAPProfile{Name: "work", Host: "imap.example.com", Username: "me"})
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Store.SetIMAPProfilePassword(ctx, prof.ID, []byte("sealed-with-session-key")); err != nil {
		t.Fatal(err)
	}
	// The session is still open when the user closes the window: its WAL may
	// hold the latest writes. Close it the way run does before the merge.
	sess.Store.Close()

	sum, err := Merge(ctx, lib, sessDir)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Added != 1 || sum.Duplicates != 1 || sum.Failed != 0 {
		t.Fatalf("summary = %+v, want 1 added, 1 duplicate", sum)
	}

	got, err := lib.Store.GetEmailBySHA(ctx, fresh)
	if err != nil {
		t.Fatalf("session mail not in library: %v", err)
	}
	if !got.Starred || !got.Read || got.CategoryID == nil {
		t.Errorf("session marks lost: starred=%v read=%v category=%v", got.Starred, got.Read, got.CategoryID)
	}
	if got.Filename != "mail-2.eml" {
		t.Errorf("filename = %q, want the session's", got.Filename)
	}
	if e, _ := lib.Store.GetEmailBySHA(ctx, shared); !e.Read {
		t.Error("mail in both lost its read mark from the library")
	}
	if cats, _ := lib.Store.ListCategories(ctx); cats[0].Name != "Mine" {
		t.Errorf("category renamed back to %q", cats[0].Name)
	}
	if v, _ := lib.Store.GetSetting(ctx, "read_tracking"); v != "on" {
		t.Errorf("library setting overwritten: %q", v)
	}
	if v, _ := lib.Store.GetSetting(ctx, "only_in_session"); v != "x" {
		t.Errorf("session-only setting not added: %q", v)
	}
	profs, _ := lib.Store.ListIMAPProfiles(ctx)
	if len(profs) != 1 || profs[0].Name != "work" || profs[0].HasPassword || profs[0].SyncEnabled {
		t.Errorf("profiles = %+v, want work without password or sync", profs)
	}

	// Running again (a retry after a failed delete) changes nothing.
	sum, err = Merge(ctx, lib, sessDir)
	if err != nil || sum.Added != 0 || sum.Duplicates != 2 {
		t.Fatalf("second merge = %+v, %v", sum, err)
	}
}

func TestMergeReportsMissingBlob(t *testing.T) {
	ctx := context.Background()
	sessDir, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sess, sp := library(t, sessDir)
	lib, _ := library(t, t.TempDir())
	sha := importEML(t, sess, 1)
	sess.Store.Close()
	if err := os.Remove(sp.BlobFor(sha)); err != nil {
		t.Fatal(err)
	}
	sum, err := Merge(ctx, lib, sessDir)
	if err == nil || sum.Failed != 1 {
		t.Fatalf("merge = %+v, %v; want a failure so the folder is kept", sum, err)
	}
}

func TestMergeEmptySession(t *testing.T) {
	sessDir, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lib, _ := library(t, t.TempDir())
	if sum, err := Merge(context.Background(), lib, sessDir); err != nil || sum != (MergeSummary{}) {
		t.Fatalf("merge = %+v, %v", sum, err)
	}
}

func TestKeptSessionsSurviveSweep(t *testing.T) {
	root := t.TempDir()
	live, err := Create(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := SetKeep(live, true); err != nil {
		t.Fatal(err)
	}
	dead := filepath.Join(root, prefix+"dead")
	if err := os.Mkdir(dead, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dead, pidFileName), []byte("2147483646"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetKeep(dead, true); err != nil {
		t.Fatal(err)
	}

	if removed := Sweep(root); len(removed) != 0 {
		t.Fatalf("swept kept sessions: %v", removed)
	}
	// Only the one whose owner is gone is ready to merge; the live one is
	// merged by its own process when it closes.
	if kept := Kept(root); len(kept) != 1 || kept[0] != dead {
		t.Fatalf("Kept = %v, want [%s]", kept, dead)
	}

	if err := SetKeep(dead, false); err != nil {
		t.Fatal(err)
	}
	if IsKept(dead) || len(Kept(root)) != 0 {
		t.Fatal("unkept session still listed")
	}
	if err := SetKeep(dead, false); err != nil {
		t.Fatalf("clearing twice: %v", err)
	}
	if removed := Sweep(root); len(removed) != 1 {
		t.Fatalf("unkept dead session not swept: %v", removed)
	}
}
