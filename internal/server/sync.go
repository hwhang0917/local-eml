package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

// ErrSyncSkipped is reported when a sync is requested for a profile that has
// no usable preconditions (e.g. password not stored, encryption unavailable).
// It is non-fatal — callers should log and move on.
var ErrSyncSkipped = errors.New("sync skipped")

// errSyncBusy is the ErrSyncSkipped a profile reports while another sync of
// it is running.
var errSyncBusy = fmt.Errorf("%w: already running", ErrSyncSkipped)

// syncLocks serializes concurrent syncs of the same profile. Two timer ticks
// landing close together, or a tick racing with a manual import, must not run
// the same profile twice.
type syncLocks struct {
	mu     sync.Mutex
	active map[int64]bool
	// rerun marks running syncs that must go once more when they end,
	// because new mail was pushed while they ran.
	rerun map[int64]bool
}

var imapSyncLocks = &syncLocks{active: map[int64]bool{}, rerun: map[int64]bool{}}

func (l *syncLocks) tryAcquire(id int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active[id] {
		return false
	}
	l.active[id] = true
	return true
}

// release ends a run and reports whether another was asked for meanwhile.
func (l *syncLocks) release(id int64) (again bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.active, id)
	again = l.rerun[id]
	delete(l.rerun, id)
	return again
}

// rerunWhenDone asks the running sync of id to run again when it ends. It
// returns false when no sync is running any more.
func (l *syncLocks) rerunWhenDone(id int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.active[id] {
		return false
	}
	l.rerun[id] = true
	return true
}

// firstSyncDelay is how soon after startup the first poll fires, so
// existing-but-stale state catches up without waiting a whole interval.
const firstSyncDelay = 5 * time.Second

// RunIMAPSync performs a single incremental fetch for the given profile,
// blocking until the underlying import job finishes. Runs are exclusive per
// profile: a second concurrent invocation returns ErrSyncSkipped.
func (s *Server) RunIMAPSync(ctx context.Context, profileID int64) error {
	if !imapSyncLocks.tryAcquire(profileID) {
		return errSyncBusy
	}
	defer func() {
		if imapSyncLocks.release(profileID) {
			go s.syncNow(profileID)
		}
	}()

	profile, err := s.Store.GetIMAPProfile(ctx, profileID)
	if err != nil {
		return err
	}
	cfg, err := s.syncConfig(ctx, profile)
	if err != nil {
		return err
	}
	cfg = withSyncCursor(cfg, profile)
	src := importer.NewIMAPSource(cfg)
	importID, err := s.queueJob(ctx, kindIMAPSync, src.Label()+" (sync)")
	if err != nil {
		return err
	}
	slog.Info("imap sync running",
		slog.Int64("profile_id", profile.ID),
		slog.String("import_id", importID),
		slog.String("host", cfg.Host),
		slog.Bool("incremental", cfg.Incremental),
		slog.Uint64("since_uid", uint64(cfg.SinceUID)),
	)

	s.runJob(importID, kindIMAPSync, src, nil, func() {
		s.persistIMAPSyncState(profile.ID, src)
	})
	return nil
}

// syncConfig builds the connection settings unattended sync uses for a
// profile, with its stored password decrypted. A profile that can't be
// synced unattended reports ErrSyncSkipped.
func (s *Server) syncConfig(ctx context.Context, profile *store.IMAPProfile) (importer.IMAPConfig, error) {
	if s.Secret == nil {
		return importer.IMAPConfig{}, fmt.Errorf("%w: encryptor not configured", ErrSyncSkipped)
	}
	if !profile.SyncEnabled {
		return importer.IMAPConfig{}, fmt.Errorf("%w: sync disabled", ErrSyncSkipped)
	}
	if !profile.HasPassword {
		return importer.IMAPConfig{}, fmt.Errorf("%w: no stored password", ErrSyncSkipped)
	}
	blob, err := s.Store.GetIMAPProfilePassword(ctx, profile.ID)
	if err != nil {
		return importer.IMAPConfig{}, fmt.Errorf("read password: %w", err)
	}
	plain, err := s.Secret.Decrypt(blob)
	if err != nil {
		return importer.IMAPConfig{}, fmt.Errorf("decrypt password: %w", err)
	}
	return profileDefaults(importer.IMAPConfig{Password: string(plain)}, profile), nil
}

// profileDefaults fills whatever cfg left blank from the saved profile.
func profileDefaults(cfg importer.IMAPConfig, p *store.IMAPProfile) importer.IMAPConfig {
	if cfg.Host == "" {
		cfg.Host = p.Host
	}
	if cfg.Port == 0 && p.Port != nil {
		cfg.Port = *p.Port
	}
	if cfg.Username == "" {
		cfg.Username = p.Username
	}
	if cfg.Folder == "" && p.Folder != nil {
		cfg.Folder = *p.Folder
	}
	return cfg
}

// withSyncCursor turns on incremental fetching when the profile carries a
// cursor from a previous run.
func withSyncCursor(cfg importer.IMAPConfig, p *store.IMAPProfile) importer.IMAPConfig {
	if p.UIDValidity != nil && p.LastUID != nil {
		cfg.Incremental = true
		cfg.ExpectedUIDValidity = *p.UIDValidity
		cfg.SinceUID = *p.LastUID
	}
	return cfg
}

// RunDueIMAPSyncs walks every sync-enabled profile and triggers a sync. It
// returns after kicking each off in sequence; per-profile errors are logged
// but never abort the loop. Profiles with a live IMAP IDLE watcher are
// polled too; see the note at the top of idle.go.
func (s *Server) RunDueIMAPSyncs(ctx context.Context) {
	profiles, err := s.Store.ListIMAPProfilesForSync(ctx)
	if err != nil {
		slog.Warn("list sync profiles failed", slog.String("err", err.Error()))
		return
	}
	for _, p := range profiles {
		if ctx.Err() != nil {
			return
		}
		if err := s.RunIMAPSync(ctx, p.ID); err != nil {
			level := slog.LevelWarn
			if errors.Is(err, ErrSyncSkipped) {
				level = slog.LevelDebug
			}
			slog.Default().Log(ctx, level, "imap sync error",
				slog.Int64("profile_id", p.ID),
				slog.String("name", p.Name),
				slog.String("err", err.Error()))
		}
	}
}

// StartIMAPSyncer launches a goroutine that polls every sync-enabled IMAP
// profile on the given interval, and keeps an IMAP IDLE watcher running for
// each so new mail is fetched as soon as the server has it. It returns
// immediately; the goroutine exits when ctx is cancelled. A duration <= 0
// starts the syncer paused — the goroutine still runs so SetIMAPSyncInterval
// can wake it later. Pausing stops the watchers too.
func (s *Server) StartIMAPSyncer(ctx context.Context, interval time.Duration) {
	s.syncIntervalNs.Store(int64(interval))
	s.syncIntervalCh = make(chan time.Duration, 1)
	s.idleKick = make(chan struct{}, 1)
	go func() {
		defer s.stopIdle()
		reconcile := time.NewTicker(idleReconcileEvery)
		defer reconcile.Stop()
		if interval > 0 {
			slog.Info("imap syncer started", slog.Duration("interval", interval))
		} else {
			slog.Info("imap syncer paused (interval off)")
		}
		timer := time.NewTimer(firstSyncDelay)
		defer timer.Stop()
		if interval <= 0 {
			timer.Stop()
		}
		s.reconcileIdle(ctx)
		for {
			select {
			case <-ctx.Done():
				slog.Info("imap syncer stopping")
				return
			case d := <-s.syncIntervalCh:
				interval = d
				timer.Stop()
				if d > 0 {
					slog.Info("imap sync interval changed", slog.Duration("interval", d))
					timer.Reset(d)
				} else {
					slog.Info("imap syncer paused")
				}
				s.reconcileIdle(ctx)
			case <-s.idleKick:
				s.reconcileIdle(ctx)
			case <-reconcile.C:
				s.reconcileIdle(ctx)
			case <-timer.C:
				s.RunDueIMAPSyncs(ctx)
				if interval > 0 {
					timer.Reset(interval)
				}
			}
		}
	}()
}

// IMAPSyncInterval reports the currently effective poll interval (0 = paused).
func (s *Server) IMAPSyncInterval() time.Duration {
	return time.Duration(s.syncIntervalNs.Load())
}

// SetIMAPSyncInterval reschedules the background poll; d <= 0 pauses it. Safe
// to call whether or not the syncer goroutine was started (tests).
func (s *Server) SetIMAPSyncInterval(d time.Duration) {
	s.syncIntervalNs.Store(int64(d))
	if s.syncIntervalCh == nil {
		return
	}
	// Buffered(1) mailbox: drop any stale pending update, then leave d in it.
	select {
	case <-s.syncIntervalCh:
	default:
	}
	s.syncIntervalCh <- d
}
