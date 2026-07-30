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

// syncLocks serializes concurrent syncs of the same profile. Two timer ticks
// landing close together, or a tick racing with a manual import, must not run
// the same profile twice.
type syncLocks struct {
	mu     sync.Mutex
	active map[int64]bool
}

var imapSyncLocks = &syncLocks{active: map[int64]bool{}}

func (l *syncLocks) tryAcquire(id int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active[id] {
		return false
	}
	l.active[id] = true
	return true
}

func (l *syncLocks) release(id int64) {
	l.mu.Lock()
	delete(l.active, id)
	l.mu.Unlock()
}

// RunIMAPSync performs a single incremental fetch for the given profile,
// blocking until the underlying import job finishes. Idempotent under
// concurrent callers — a second concurrent invocation returns ErrSyncSkipped.
func (s *Server) RunIMAPSync(ctx context.Context, profileID int64) error {
	if !imapSyncLocks.tryAcquire(profileID) {
		return fmt.Errorf("%w: already running", ErrSyncSkipped)
	}
	defer imapSyncLocks.release(profileID)

	if s.Secret == nil {
		return fmt.Errorf("%w: encryptor not configured", ErrSyncSkipped)
	}
	profile, err := s.Store.GetIMAPProfile(ctx, profileID)
	if err != nil {
		return err
	}
	if !profile.SyncEnabled {
		return fmt.Errorf("%w: sync disabled", ErrSyncSkipped)
	}
	if !profile.HasPassword {
		return fmt.Errorf("%w: no stored password", ErrSyncSkipped)
	}
	blob, err := s.Store.GetIMAPProfilePassword(ctx, profileID)
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	plain, err := s.Secret.Decrypt(blob)
	if err != nil {
		return fmt.Errorf("decrypt password: %w", err)
	}

	cfg := importer.IMAPConfig{
		Host:     profile.Host,
		Username: profile.Username,
		Password: string(plain),
	}
	if profile.Port != nil {
		cfg.Port = *profile.Port
	}
	if profile.Folder != nil {
		cfg.Folder = *profile.Folder
	}
	if profile.UIDValidity != nil && profile.LastUID != nil {
		cfg.Incremental = true
		cfg.ExpectedUIDValidity = *profile.UIDValidity
		cfg.SinceUID = *profile.LastUID
	}

	folder := cfg.Folder
	if folder == "" {
		folder = "INBOX"
	}
	importID := newImportID()
	sourceName := fmt.Sprintf("imap://%s@%s/%s (sync)", cfg.Username, cfg.Host, folder)
	if err := s.Store.CreateImport(ctx, store.Import{
		ID:         importID,
		SourceKind: "imap-sync",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		return fmt.Errorf("create import: %w", err)
	}

	src := importer.NewIMAPSource(cfg)
	slog.Info("imap sync running",
		slog.Int64("profile_id", profile.ID),
		slog.String("import_id", importID),
		slog.String("host", cfg.Host),
		slog.Bool("incremental", cfg.Incremental),
		slog.Uint64("since_uid", uint64(cfg.SinceUID)),
	)

	s.runJob(importID, "imap-sync", src, func() {}, func() {
		s.persistIMAPSyncState(profile.ID, src)
	})
	return nil
}

// RunDueIMAPSyncs walks every sync-enabled profile and triggers a sync. It
// returns after kicking each off in sequence; per-profile errors are logged
// but never abort the loop.
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
// profile on the given interval. It returns immediately; the goroutine exits
// when ctx is cancelled. A duration <= 0 starts the syncer paused — the
// goroutine still runs so SetIMAPSyncInterval can wake it later.
func (s *Server) StartIMAPSyncer(ctx context.Context, interval time.Duration) {
	s.syncIntervalNs.Store(int64(interval))
	s.syncIntervalCh = make(chan time.Duration, 1)
	go func() {
		if interval > 0 {
			slog.Info("imap syncer started", slog.Duration("interval", interval))
		} else {
			slog.Info("imap syncer paused (interval off)")
		}
		// First fire lands shortly after startup so existing-but-stale state
		// catches up without waiting a whole interval.
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		if interval <= 0 {
			timer.Stop()
		}
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
