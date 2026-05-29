package importer

import (
	"context"
	"errors"
	"io"
	"testing"

	imap "github.com/emersion/go-imap/v2"
)

type fakeIMAP struct {
	uids        []imap.UID
	bodies      map[imap.UID]string
	fetchErr    map[imap.UID]error
	uidsErr     error
	uidValidity uint32
	lastMinUID  uint32
	closed      bool
}

func (f *fakeIMAP) UIDValidity() uint32 { return f.uidValidity }

func (f *fakeIMAP) UIDs(minUID uint32) ([]imap.UID, error) {
	f.lastMinUID = minUID
	if f.uidsErr != nil {
		return nil, f.uidsErr
	}
	if minUID == 0 {
		return f.uids, nil
	}
	var out []imap.UID
	for _, u := range f.uids {
		if uint32(u) >= minUID {
			out = append(out, u)
		}
	}
	return out, nil
}

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

func TestIMAPSourceIncrementalUsesSinceUID(t *testing.T) {
	f := &fakeIMAP{
		uids:        []imap.UID{42, 100},
		bodies:      map[imap.UID]string{42: "old", 100: "new"},
		uidValidity: 7,
	}
	src := sourceWithFake(f, IMAPConfig{
		Host: "h", Username: "u",
		Incremental: true, ExpectedUIDValidity: 7, SinceUID: 50,
	})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.lastMinUID != 51 {
		t.Errorf("lastMinUID = %d, want 51 (SinceUID+1)", f.lastMinUID)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item (uid 100 only), got %d", len(items))
	}
	res, ok := src.SyncResult()
	if !ok {
		t.Fatalf("expected SyncResult ok")
	}
	if res.UIDValidity != 7 || res.MaxUID != 100 {
		t.Errorf("result = %+v, want {UIDValidity:7 MaxUID:100}", res)
	}
}

func TestIMAPSourceUIDValidityMismatchFallsBackToFullScan(t *testing.T) {
	f := &fakeIMAP{
		uids:        []imap.UID{1, 2, 3},
		bodies:      map[imap.UID]string{1: "a", 2: "b", 3: "c"},
		uidValidity: 999, // server moved
	}
	src := sourceWithFake(f, IMAPConfig{
		Host: "h", Username: "u",
		Incremental: true, ExpectedUIDValidity: 7, SinceUID: 50,
	})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.lastMinUID != 0 {
		t.Errorf("lastMinUID = %d, want 0 (full scan after UIDVALIDITY mismatch)", f.lastMinUID)
	}
	if len(items) != 3 {
		t.Errorf("want 3 items after fallback, got %d", len(items))
	}
}

func TestIMAPSourceEmptyIncrementalKeepsSinceUID(t *testing.T) {
	f := &fakeIMAP{uidValidity: 9}
	src := sourceWithFake(f, IMAPConfig{
		Host: "h", Username: "u",
		Incremental: true, ExpectedUIDValidity: 9, SinceUID: 200,
	})
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("want 0 items, got %d", len(items))
	}
	res, ok := src.SyncResult()
	if !ok || res.MaxUID != 200 {
		t.Errorf("result = %+v ok=%v, want MaxUID=200 (preserve since)", res, ok)
	}
}

func TestIMAPSourceScanClosesSessionOnUIDsError(t *testing.T) {
	f := &fakeIMAP{uidsErr: errors.New("search failed")}
	src := sourceWithFake(f, IMAPConfig{Host: "h", Username: "u"})
	if _, err := src.Scan(context.Background()); err == nil {
		t.Fatal("expected Scan error, got nil")
	}
	if !f.closed {
		t.Error("session not closed after UIDs error (leak)")
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
