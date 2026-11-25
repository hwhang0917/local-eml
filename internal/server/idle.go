package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hwhang0917/local-eml/internal/importer"
)

// IMAP IDLE push. Every profile that background sync may log into gets one
// long-lived connection idling on its folder; when the server says mail
// arrived, the profile is synced straight away. The interval poll keeps
// running beside it at the interval the user picked, whatever IDLE reports:
// some servers quietly stop answering an IDLE (one seen in the wild ends it on
// its own and then rejects the DONE), and a host that sleeps leaves a
// connection that looks live but is gone. "Live" is never proof that mail
// will arrive, so it must not hold the poll back.

// idleReconcileEvery is how often the set of watchers is compared with the
// saved profiles, on top of the explicit kicks after a profile changes. It
// catches changes made by paths that don't kick (a library wipe, say).
const idleReconcileEvery = time.Minute

// idleWatchers tracks the running watchers by profile id. Its zero value is
// ready to use.
type idleWatchers struct {
	mu sync.Mutex
	m  map[int64]*idleWatch
}

type idleWatch struct {
	// key is what the connection was opened with; a different key (new host,
	// folder or password) means the watcher must be restarted.
	key    string
	cancel context.CancelFunc
	state  importer.IdleState
}

// idleTarget is one profile that should be watched.
type idleTarget struct {
	key string
	cfg importer.IMAPConfig
}

// pushState reports a profile's push state for the profile cards: "" when no
// watcher runs for it.
func (s *Server) pushState(id int64) importer.IdleState {
	s.idle.mu.Lock()
	defer s.idle.mu.Unlock()
	if w := s.idle.m[id]; w != nil {
		return w.state
	}
	return ""
}

// kickIdle asks the syncer to reconcile watchers soon, e.g. after a profile
// was saved. It never blocks.
func (s *Server) kickIdle() {
	if s.idleKick == nil {
		return
	}
	select {
	case s.idleKick <- struct{}{}:
	default:
	}
}

// reconcileIdle starts a watcher for every profile that should have one and
// stops the rest. Background sync switched off (interval 0) stops them all.
func (s *Server) reconcileIdle(ctx context.Context) {
	want := s.idleTargets(ctx)

	s.idle.mu.Lock()
	defer s.idle.mu.Unlock()
	if s.idle.m == nil {
		s.idle.m = map[int64]*idleWatch{}
	}
	for id, w := range s.idle.m {
		if t, ok := want[id]; !ok || t.key != w.key {
			w.cancel()
			delete(s.idle.m, id)
		}
	}
	for id, t := range want {
		if _, ok := s.idle.m[id]; ok {
			continue
		}
		wctx, cancel := context.WithCancel(ctx)
		w := &idleWatch{key: t.key, cancel: cancel, state: importer.IdleConnecting}
		s.idle.m[id] = w
		go s.runIdle(wctx, id, w, t.cfg)
	}
}

// stopIdle stops every watcher.
func (s *Server) stopIdle() {
	s.idle.mu.Lock()
	defer s.idle.mu.Unlock()
	for id, w := range s.idle.m {
		w.cancel()
		delete(s.idle.m, id)
	}
}

func (s *Server) idleTargets(ctx context.Context) map[int64]idleTarget {
	want := map[int64]idleTarget{}
	if s.Secret == nil || s.IMAPSyncInterval() <= 0 {
		return want
	}
	profiles, err := s.Store.ListIMAPProfilesForSync(ctx)
	if err != nil {
		slog.Warn("list push profiles failed", slog.String("err", err.Error()))
		return want
	}
	for i := range profiles {
		p := &profiles[i]
		cfg, err := s.syncConfig(ctx, p)
		if err != nil {
			slog.Debug("imap push skipped", slog.Int64("profile_id", p.ID), slog.String("err", err.Error()))
			continue
		}
		want[p.ID] = idleTarget{key: idleKey(cfg), cfg: cfg}
	}
	return want
}

func idleKey(cfg importer.IMAPConfig) string {
	h := sha256.Sum256([]byte(cfg.Password))
	return fmt.Sprintf("%s|%d|%s|%s|%s", cfg.Host, cfg.Port, cfg.Username, cfg.Mailbox(), hex.EncodeToString(h[:8]))
}

// runIdle drives one watcher until its context ends or the server turns out
// not to support IDLE. In the latter case the entry stays, marked
// unsupported, so reconcile doesn't redial it every minute; a profile change
// clears it.
func (s *Server) runIdle(ctx context.Context, id int64, w *idleWatch, cfg importer.IMAPConfig) {
	log := slog.Default().With(slog.Int64("profile_id", id), slog.String("host", cfg.Host))
	run := s.idleRunner
	if run == nil {
		run = func(ctx context.Context, cfg importer.IMAPConfig, onChange func(), onState func(importer.IdleState)) error {
			watcher := &importer.IdleWatcher{Config: cfg, OnChange: onChange, OnState: onState, Logger: log}
			return watcher.Run(ctx)
		}
	}
	onState := func(st importer.IdleState) {
		s.idle.mu.Lock()
		w.state = st
		s.idle.mu.Unlock()
	}
	onChange := func() { go s.syncNow(id) }
	err := run(ctx, cfg, onChange, onState)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, importer.ErrIdleUnsupported) {
		log.Warn("imap push stopped", slog.String("err", err.Error()))
	}
}

// syncNow syncs a profile because its server pushed new mail. If a sync is
// already running, it is asked to run once more when it ends rather than
// dropping the push: the running one may have searched before the new
// message landed.
func (s *Server) syncNow(id int64) {
	for range 2 {
		err := s.RunIMAPSync(context.Background(), id)
		if !errors.Is(err, errSyncBusy) {
			if err != nil && !errors.Is(err, ErrSyncSkipped) {
				slog.Warn("imap push sync failed", slog.Int64("profile_id", id), slog.String("err", err.Error()))
			}
			return
		}
		if imapSyncLocks.rerunWhenDone(id) {
			return
		}
		// The running sync ended between the two calls; try again ourselves.
	}
}
