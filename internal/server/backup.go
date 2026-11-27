package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hwhang0917/local-eml/internal/exporter"
)

// checkBackupDir is swappable in tests.
var checkBackupDir = exporter.CheckBackupDir

// Automatic backups copy the library into a folder (an external drive, a NAS
// share, a Dropbox or OneDrive folder) on a schedule. After "Free up Gmail"
// this archive may be the only copy of years of mail; one dead disk should
// not be able to take it.

const (
	BackupConfigSettingKey = "backup"
	BackupLastSettingKey   = "backup_last"

	backupOff    = "off"
	backupDaily  = "daily"
	backupWeekly = "weekly"
)

// How often the scheduler looks, how soon after start, and how long a failed
// run waits before trying again (an unplugged drive, say).
var (
	backupCheckEvery = 10 * time.Minute
	backupFirstCheck = 2 * time.Minute
	backupRetryAfter = time.Hour
)

type backupConfig struct {
	Dir   string `json:"dir"`
	Every string `json:"every"`
}

func (c backupConfig) period() time.Duration {
	switch c.Every {
	case backupDaily:
		return 24 * time.Hour
	case backupWeekly:
		return 7 * 24 * time.Hour
	}
	return 0
}

// backupLast records the latest attempt and the latest success separately, so
// a failing drive shows both "failed just now" and "last good backup: Monday".
type backupLast struct {
	At      int64  `json:"at,omitempty"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Added   int    `json:"added"`
	Total   int    `json:"total"`
	Failed  int    `json:"failed"`
	Seconds int64  `json:"seconds"`
	// LastOK is when the last successful run finished.
	LastOK int64 `json:"last_ok,omitempty"`
}

type backupRunner struct {
	mu      sync.Mutex
	running bool
}

func (b *backupRunner) tryStart() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.running {
		return false
	}
	b.running = true
	return true
}

func (b *backupRunner) done() {
	b.mu.Lock()
	b.running = false
	b.mu.Unlock()
}

func (b *backupRunner) isRunning() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.running
}

func readSettingJSON[T any](ctx context.Context, s *Server, key string) (T, error) {
	var v T
	raw, err := s.Store.GetSetting(ctx, key)
	if err != nil || raw == "" {
		return v, err
	}
	_ = json.Unmarshal([]byte(raw), &v)
	return v, nil
}

func writeSettingJSON(ctx context.Context, s *Server, key string, v any) error {
	b, _ := json.Marshal(v)
	return s.Store.SetSetting(ctx, key, string(b))
}

func (s *Server) backupConfig(ctx context.Context) (backupConfig, error) {
	c, err := readSettingJSON[backupConfig](ctx, s, BackupConfigSettingKey)
	if c.Every == "" {
		c.Every = backupOff
	}
	return c, err
}

// runBackup does one backup into the configured folder and records it.
func (s *Server) runBackup(ctx context.Context, cfg backupConfig) {
	defer s.backup.done()
	log := slog.Default().With(slog.String("dir", cfg.Dir))
	log.Info("backup started")
	res, err := s.Exporter.BackupToFolder(ctx, cfg.Dir)

	prev, _ := readSettingJSON[backupLast](ctx, s, BackupLastSettingKey)
	last := backupLast{
		At: time.Now().Unix(), OK: err == nil,
		Added: res.Added, Total: res.Total, Failed: res.Failed,
		Seconds: int64(res.Elapsed.Seconds()), LastOK: prev.LastOK,
	}
	if err != nil {
		last.Error = err.Error()
		log.Error("backup failed", slog.String("err", err.Error()))
	} else {
		last.LastOK = last.At
		log.Info("backup finished", slog.Int("added", res.Added), slog.Int("total", res.Total),
			slog.Int("failed", res.Failed), slog.Duration("elapsed", res.Elapsed))
	}
	if werr := writeSettingJSON(context.Background(), s, BackupLastSettingKey, last); werr != nil {
		log.Warn("record backup result", slog.String("err", werr.Error()))
	}
}

// backupDue reports whether a scheduled run should start now.
func backupDue(cfg backupConfig, last backupLast, now time.Time) bool {
	p := cfg.period()
	if p == 0 || cfg.Dir == "" {
		return false
	}
	if last.At == 0 {
		return true
	}
	since := now.Sub(time.Unix(last.At, 0))
	if !last.OK {
		return since >= backupRetryAfter
	}
	return since >= p
}

// StartBackupScheduler checks now and then whether a backup is due. It runs
// whenever Local Eml does: all the time as a service, while the window is
// open otherwise, and a backup missed while it was closed runs soon after it
// next starts.
func (s *Server) StartBackupScheduler(ctx context.Context) {
	if s.TemporarySession {
		return
	}
	go func() {
		timer := time.NewTimer(backupFirstCheck)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				s.maybeBackup(ctx)
				timer.Reset(backupCheckEvery)
			}
		}
	}()
}

func (s *Server) maybeBackup(ctx context.Context) {
	cfg, err := s.backupConfig(ctx)
	if err != nil {
		return
	}
	last, _ := readSettingJSON[backupLast](ctx, s, BackupLastSettingKey)
	if !backupDue(cfg, last, time.Now()) || !s.backup.tryStart() {
		return
	}
	s.runBackup(ctx, cfg)
}

type backupState struct {
	backupConfig
	Last    backupLast `json:"last"`
	Running bool       `json:"running"`
	// DefaultDir is a suggestion for the folder field.
	DefaultDir string `json:"default_dir"`
	// Available is false in a temporary session, which backs nothing up.
	Available bool `json:"available"`
}

func (s *Server) handleGetBackup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg, err := s.backupConfig(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	last, _ := readSettingJSON[backupLast](ctx, s, BackupLastSettingKey)
	def := ""
	if home, err := os.UserHomeDir(); err == nil {
		def = filepath.Join(home, "Local Eml Backup")
	}
	writeJSON(w, http.StatusOK, backupState{
		backupConfig: cfg, Last: last, Running: s.backup.isRunning(),
		DefaultDir: def, Available: !s.TemporarySession,
	})
}

func (s *Server) handlePutBackup(w http.ResponseWriter, r *http.Request) {
	if s.TemporarySession {
		http.Error(w, "backups are off in a temporary session", http.StatusConflict)
		return
	}
	var c backupConfig
	if !decodeJSON(w, r, &c) {
		return
	}
	c.Dir = strings.TrimSpace(c.Dir)
	switch c.Every {
	case backupOff, backupDaily, backupWeekly:
	default:
		http.Error(w, `every must be "off", "daily" or "weekly"`, http.StatusBadRequest)
		return
	}
	if c.Every != backupOff || c.Dir != "" {
		if c.Dir == "" {
			http.Error(w, "choose a folder to back up to", http.StatusBadRequest)
			return
		}
		c.Dir = filepath.Clean(c.Dir)
		if err := checkBackupDir(c.Dir); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err := writeSettingJSON(r.Context(), s, BackupConfigSettingKey, c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleGetBackup(w, r)
}

// handleRunBackup starts a backup now, whatever the schedule says.
func (s *Server) handleRunBackup(w http.ResponseWriter, r *http.Request) {
	if s.TemporarySession {
		http.Error(w, "backups are off in a temporary session", http.StatusConflict)
		return
	}
	cfg, err := s.backupConfig(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if cfg.Dir == "" {
		http.Error(w, "choose a folder to back up to first", http.StatusBadRequest)
		return
	}
	if !s.backup.tryStart() {
		http.Error(w, "a backup is already running", http.StatusConflict)
		return
	}
	go s.runBackup(context.Background(), cfg)
	w.WriteHeader(http.StatusAccepted)
}
