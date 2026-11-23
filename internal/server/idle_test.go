package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/secret"
	"github.com/hwhang0917/local-eml/internal/store"
)

// fakeIdle stands in for the real IDLE watcher: it reports live, records
// which configs were started, and blocks until stopped.
type fakeIdle struct {
	mu      sync.Mutex
	started []importer.IMAPConfig
	running int
}

func (f *fakeIdle) run(ctx context.Context, cfg importer.IMAPConfig, _ func(), onState func(importer.IdleState)) error {
	f.mu.Lock()
	f.started = append(f.started, cfg)
	f.running++
	f.mu.Unlock()
	onState(importer.IdleLive)
	<-ctx.Done()
	f.mu.Lock()
	f.running--
	f.mu.Unlock()
	return ctx.Err()
}

func (f *fakeIdle) counts() (started, running int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.started), f.running
}

func newPushServer(t *testing.T) (*Server, *fakeIdle) {
	t.Helper()
	s, p := newDriftServer(t)
	enc, err := secret.Open(p.KeyFile())
	if err != nil {
		t.Fatal(err)
	}
	s.Secret = enc
	f := &fakeIdle{}
	s.idleRunner = f.run
	s.syncIntervalNs.Store(int64(5 * time.Minute))
	return s, f
}

func saveSyncProfile(t *testing.T, s *Server, name, password string, syncOn bool) store.IMAPProfile {
	t.Helper()
	ctx := context.Background()
	prof, err := s.Store.UpsertIMAPProfile(ctx, store.IMAPProfile{
		Name: name, Host: "imap.example.com", Username: name + "@example.com", SyncEnabled: syncOn,
	})
	if err != nil {
		t.Fatal(err)
	}
	if password != "" {
		blob, err := s.Secret.Encrypt([]byte(password))
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Store.SetIMAPProfilePassword(ctx, prof.ID, blob); err != nil {
			t.Fatal(err)
		}
	}
	return prof
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestReconcileIdleWatchesSyncableProfilesOnly(t *testing.T) {
	s, f := newPushServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	work := saveSyncProfile(t, s, "work", "pw1", true)
	off := saveSyncProfile(t, s, "off", "pw2", false)
	nopw := saveSyncProfile(t, s, "nopw", "", true)

	s.reconcileIdle(ctx)
	eventually(t, "one watcher", func() bool { _, r := f.counts(); return r == 1 })
	eventually(t, "live state", func() bool { return s.pushState(work.ID) == importer.IdleLive })
	if s.pushState(off.ID) != "" || s.pushState(nopw.ID) != "" {
		t.Fatal("watcher started for a profile background sync can't use")
	}

	// The profile list reports it to the cards.
	rec := doJSON(t, s, http.MethodGet, "/api/imap/profiles", "")
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	push := map[string]any{}
	for _, p := range list {
		push[p["name"].(string)] = p["push"]
	}
	if push["work"] != "live" || push["off"] != nil {
		t.Fatalf("push states = %v", push)
	}

	// Reconciling again with nothing changed keeps the same connection.
	s.reconcileIdle(ctx)
	if started, _ := f.counts(); started != 1 {
		t.Fatalf("started %d watchers, want 1", started)
	}

	// A new password means a new login: the watcher restarts.
	blob, _ := s.Secret.Encrypt([]byte("pw-new"))
	_ = s.Store.SetIMAPProfilePassword(ctx, work.ID, blob)
	s.reconcileIdle(ctx)
	eventually(t, "restart", func() bool { st, r := f.counts(); return st == 2 && r == 1 })

	// Pausing background sync closes every push connection.
	s.syncIntervalNs.Store(0)
	s.reconcileIdle(ctx)
	eventually(t, "all stopped", func() bool { _, r := f.counts(); return r == 0 })
	if s.pushState(work.ID) != "" {
		t.Fatal("paused profile still reports a push state")
	}
}

func TestPushCoveredSkipsPollOnlyWhileFresh(t *testing.T) {
	s, _ := newPushServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	prof := saveSyncProfile(t, s, "work", "pw", true)

	recent := time.Now().Add(-time.Minute).Unix()
	stale := time.Now().Add(-2 * idleSafetyPoll).Unix()
	prof.LastSyncedAt = &recent
	if s.pushCovered(&prof) {
		t.Fatal("covered before any watcher runs")
	}
	s.reconcileIdle(ctx)
	eventually(t, "live", func() bool { return s.pushState(prof.ID) == importer.IdleLive })
	if !s.pushCovered(&prof) {
		t.Fatal("live and freshly synced profile should be left to push")
	}
	prof.LastSyncedAt = &stale
	if s.pushCovered(&prof) {
		t.Fatal("stale profile should still get the safety poll")
	}
}

func TestSyncLocksRerunWhenDone(t *testing.T) {
	l := &syncLocks{active: map[int64]bool{}, rerun: map[int64]bool{}}
	if l.rerunWhenDone(1) {
		t.Fatal("rerun accepted with nothing running")
	}
	if !l.tryAcquire(1) {
		t.Fatal("acquire failed")
	}
	if !l.rerunWhenDone(1) {
		t.Fatal("rerun refused while running")
	}
	if !l.release(1) {
		t.Fatal("release lost the rerun request")
	}
	if !l.tryAcquire(1) || l.release(1) {
		t.Fatal("rerun request should be consumed once")
	}
}

func TestMailEventsStreamAnnouncesNewMail(t *testing.T) {
	s, _ := newPushServer(t)
	srv := httptest.NewServer(s.Router())
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/events", nil)
	req.Header.Set("Authorization", bearerPrefix+testToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if ct := res.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content type %q", ct)
	}
	lines := bufio.NewScanner(res.Body)
	next := func() mailEvent {
		t.Helper()
		for lines.Scan() {
			if data, ok := strings.CutPrefix(lines.Text(), "data: "); ok {
				var ev mailEvent
				if err := json.Unmarshal([]byte(data), &ev); err != nil {
					t.Fatal(err)
				}
				return ev
			}
		}
		t.Fatalf("stream ended: %v", lines.Err())
		return mailEvent{}
	}
	if ev := next(); ev.Type != mailEventReady {
		t.Fatalf("first event %q, want ready", ev.Type)
	}

	ctx := context.Background()
	// A sync that only saw known mail stays quiet; one that added mail speaks.
	quiet, _ := s.queueJob(ctx, kindIMAPSync, "quiet")
	_ = s.Store.IncImportCounters(ctx, quiet, 2, 2, 0)
	s.announceImported(quiet, kindIMAPSync)
	fresh, _ := s.queueJob(ctx, kindIMAPSync, "fresh")
	_ = s.Store.IncImportCounters(ctx, fresh, 4, 1, 0)
	s.announceImported(fresh, kindIMAPSync)

	ev := next()
	if ev.Type != mailEventNewMail || ev.ImportID != fresh || ev.Added != 3 || ev.Kind != kindIMAPSync {
		t.Fatalf("event = %+v, want new-mail for %s with 3 added", ev, fresh)
	}
}
