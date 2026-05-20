package importer

import (
	"context"
	"errors"
	"io"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

type fakeIMAP struct {
	uids     []imap.UID
	bodies   map[imap.UID]string
	fetchErr map[imap.UID]error
	closed   bool
}

func (f *fakeIMAP) UIDs() ([]imap.UID, error) { return f.uids, nil }

func (f *fakeIMAP) Fetch(uid imap.UID) ([]byte, error) {
	if f.fetchErr != nil {
		if err := f.fetchErr[uid]; err != nil {
			return nil, err
		}
	}
	return []byte(f.bodies[uid]), nil
}

func (f *fakeIMAP) Close() error { f.closed = true; return nil }

func sourceWithFake(f *fakeIMAP, cfg IMAPConfig) *imapSource {
	return &imapSource{cfg: cfg, dial: func(IMAPConfig) (imapSession, error) { return f, nil }}
}

func TestIMAPSourceScanMapsUIDsToItems(t *testing.T) {
	f := &fakeIMAP{
		uids:   []imap.UID{7, 9},
		bodies: map[imap.UID]string{7: "SEVEN", 9: "NINE"},
	}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items, got %d", len(items))
	}
	rc, err := items[0].Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "SEVEN" {
		t.Errorf("body = %q, want SEVEN", string(b))
	}
}

func TestIMAPSourceFetchErrorSurfaces(t *testing.T) {
	f := &fakeIMAP{
		uids:     []imap.UID{1},
		fetchErr: map[imap.UID]error{1: errors.New("boom")},
	}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := items[0].Open(context.Background()); err == nil {
		t.Error("expected Open error, got nil")
	}
}

func TestIMAPSourceLabel(t *testing.T) {
	def := &imapSource{cfg: IMAPConfig{Host: "mail.example.com", Username: "alice"}}
	if got := def.Label(); got != "imap://alice@mail.example.com/INBOX" {
		t.Errorf("label = %q", got)
	}
	named := &imapSource{cfg: IMAPConfig{Host: "h", Username: "u", Folder: "Archive"}}
	if got := named.Label(); got != "imap://u@h/Archive" {
		t.Errorf("label = %q", got)
	}
}

func TestIMAPSourceCloseClosesSession(t *testing.T) {
	f := &fakeIMAP{uids: []imap.UID{1}, bodies: map[imap.UID]string{1: "x"}}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	if _, err := src.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
	if !f.closed {
		t.Error("Close did not close the session")
	}
}

func TestRunSourceDrivesIMAPSource(t *testing.T) {
	st := newTestStore(t)
	im := &Importer{Store: st, Paths: newTestPaths(t)}
	hub := NewHub()

	importID := "imap1"
	if err := st.CreateImport(context.Background(), importStub(importID)); err != nil {
		t.Fatal(err)
	}

	f := &fakeIMAP{
		uids:     []imap.UID{1, 2},
		bodies:   map[imap.UID]string{1: minimalEML},
		fetchErr: map[imap.UID]error{2: errors.New("nope")},
	}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})

	job := &Job{Importer: im, Hub: hub, Store: st, ID: importID}
	job.RunSource(context.Background(), src)

	imp, err := st.GetImport(context.Background(), importID)
	if err != nil {
		t.Fatal(err)
	}
	if imp.Status != "done" {
		t.Errorf("status = %q, want done", imp.Status)
	}
	if imp.Total != 2 || imp.Processed != 2 || imp.Errors != 1 {
		t.Errorf("counters total=%d processed=%d errors=%d, want 2/2/1",
			imp.Total, imp.Processed, imp.Errors)
	}
}
