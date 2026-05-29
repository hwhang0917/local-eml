package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hwhang0917/local-eml/internal/updater"
)

// updateCache memoizes the GitHub release lookup so a busy About page doesn't
// hammer api.github.com (60 req/h unauth). Cache entries are invalidated by
// time; a manual force=1 in the check request bypasses them.
type updateCache struct {
	mu        sync.Mutex
	fetchedAt time.Time
	release   *updater.Release
	err       error
}

const updateCacheTTL = 10 * time.Minute

var updateChecker = &updateCache{}

func (c *updateCache) get(ctx context.Context, force bool) (*updater.Release, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.release != nil && time.Since(c.fetchedAt) < updateCacheTTL {
		return c.release, c.err
	}
	rel, err := updater.LatestRelease(ctx)
	c.fetchedAt = time.Now()
	c.release = rel
	c.err = err
	return rel, err
}

type updateStatus struct {
	Current     string `json:"current"`
	Latest      string `json:"latest,omitempty"`
	HasUpdate   bool   `json:"has_update"`
	ReleaseURL  string `json:"release_url,omitempty"`
	Notes       string `json:"notes,omitempty"`
	AssetName   string `json:"asset_name,omitempty"`
	CanInstall  bool   `json:"can_install"`
	InstallNote string `json:"install_note,omitempty"`
	Error       string `json:"error,omitempty"`
}

func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "1"
	rel, err := updateChecker.get(r.Context(), force)
	status := updateStatus{
		Current:     s.Version,
		AssetName:   updater.AssetName(),
		CanInstall:  s.canSelfInstall(),
		InstallNote: s.installNote(),
	}
	if err != nil {
		status.Error = err.Error()
		writeJSON(w, http.StatusOK, status)
		return
	}
	status.Latest = rel.Tag
	status.ReleaseURL = rel.URL
	status.Notes = rel.Body
	status.HasUpdate = updater.IsNewer(s.Version, rel.Tag)
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleUpdateInstall(w http.ResponseWriter, r *http.Request) {
	if !s.canSelfInstall() {
		http.Error(w, s.installNote(), http.StatusBadRequest)
		return
	}

	rel, err := updateChecker.get(r.Context(), true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if !updater.IsNewer(s.Version, rel.Tag) {
		http.Error(w, "already on the latest version", http.StatusConflict)
		return
	}
	binAsset, sumsAsset, err := rel.FindAsset()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	sums, err := updater.FetchChecksums(r.Context(), sumsAsset.DownloadURL)
	if err != nil {
		http.Error(w, "fetch checksums: "+err.Error(), http.StatusBadGateway)
		return
	}
	expectedSHA, ok := sums[binAsset.Name]
	if !ok {
		http.Error(w, "no checksum for "+binAsset.Name, http.StatusBadGateway)
		return
	}

	currentPath, err := updater.CurrentBinary()
	if err != nil {
		http.Error(w, "resolve current binary: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tmp, err := updater.Download(r.Context(), binAsset.DownloadURL, expectedSHA, filepath.Dir(currentPath))
	if err != nil {
		http.Error(w, "download: "+err.Error(), http.StatusBadGateway)
		return
	}
	if err := updater.Swap(currentPath, tmp); err != nil {
		os.Remove(tmp)
		http.Error(w, "swap binary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	slog.Info("update installed",
		slog.String("from", s.Version),
		slog.String("to", rel.Tag),
		slog.String("binary", currentPath),
	)

	// Reply before triggering restart so the client sees the success and
	// starts polling /healthz instead of treating EOF as a generic failure.
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":     "installed",
		"from":       s.Version,
		"to":         rel.Tag,
		"restarting": true,
	})

	// On Linux/systemd, the auto-restart-after-exit path waits RestartSec
	// (kardianos's default unit hardcodes RestartSec=120) — so a plain
	// os.Exit would leave the server down for 2 minutes. Issue an explicit
	// out-of-band restart instead; it bypasses RestartSec and tells systemd
	// to bring us back up the moment our process exits. On macOS/launchd,
	// KeepAlive already restarts within ~1s of exit so the nudge is a no-op.
	go func() {
		time.Sleep(250 * time.Millisecond)
		if err := requestImmediateRestart(); err != nil {
			slog.Warn("immediate restart request failed; falling back to service-manager auto-restart-on-exit",
				slog.String("err", err.Error()))
		} else {
			slog.Info("queued explicit service-manager restart")
		}
		// Hold open briefly so the service manager's SIGTERM (sent as part
		// of the queued restart) can drive the graceful Shutdown path. If
		// no signal arrives, exit anyway — the auto-restart policy of the
		// platform takes over.
		time.Sleep(5 * time.Second)
		slog.Info("no restart signal received within deadline; exiting")
		os.Exit(0)
	}()
}

// canSelfInstall reports whether we have a managed service to bring us back
// up after we exit. Without one (e.g. when run as `./local-eml serve` from a
// terminal), installing an update would leave the server down until the user
// restarts it manually — we refuse and tell them.
func (s *Server) canSelfInstall() bool {
	return !s.RunningInteractive
}

func (s *Server) installNote() string {
	if !s.canSelfInstall() {
		return "Updates can only be installed when running as a managed service. Run `local-eml install` first."
	}
	return ""
}

