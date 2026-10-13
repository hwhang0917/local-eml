package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hwhang0917/local-eml/internal/prefs"
)

// Session modes, as reported to the Settings page. Service and temporary are
// mutually exclusive: a service never runs in a throwaway folder.
const (
	ModeService   = "service"   // started by the service manager; never "closes"
	ModeTemporary = "temporary" // throwaway folder, deleted on exit
	ModeNormal    = "normal"    // double-click or terminal run on the real library
)

func (s *Server) mode() string {
	switch {
	case !s.RunningInteractive:
		return ModeService
	case s.TemporarySession:
		return ModeTemporary
	default:
		return ModeNormal
	}
}

// GET /api/session
func (s *Server) handleSession(w http.ResponseWriter, _ *http.Request) {
	var p prefs.Prefs
	if s.PrefsFile != "" {
		var err error
		if p, err = prefs.Load(s.PrefsFile); err != nil {
			slog.Warn("read prefs", "err", err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":               s.mode(),
		"temporary_launches": p.TemporaryLaunches,
	})
}

// PUT /api/session/temporary-launches  {"enabled": bool}
func (s *Server) handlePutTemporaryLaunches(w http.ResponseWriter, r *http.Request) {
	// The page a double-click opens may be served by the running service; the
	// switch would then land in the service's folder and govern nothing.
	if s.mode() == ModeService {
		http.Error(w, "temporary sessions are not available while running as a service", http.StatusConflict)
		return
	}
	if s.PrefsFile == "" {
		http.Error(w, "preferences are not configured", http.StatusInternalServerError)
		return
	}
	var body struct {
		Enabled *bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Enabled == nil {
		http.Error(w, `body must be {"enabled": true|false}`, http.StatusBadRequest)
		return
	}
	p, err := prefs.Load(s.PrefsFile)
	if err != nil {
		slog.Warn("read prefs; overwriting", "err", err)
	}
	p.TemporaryLaunches = *body.Enabled
	if err := prefs.Save(s.PrefsFile, p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/data/wipe deletes all mail, the search index, import history and
// logs, and turns background sync off. Profiles, categories, settings and the
// encryption key are kept.
func (s *Server) handleWipeData(w http.ResponseWriter, r *http.Request) {
	// A running import would write mail straight back in. Refusing is simpler
	// and more honest than racing it.
	if s.Canceller != nil && s.Canceller.Active() > 0 {
		http.Error(w, "an import or export is running; cancel it first", http.StatusConflict)
		return
	}
	n, err := s.Store.WipeLibrary(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p := s.Importer.Paths
	if err := emptyDir(p.EML, os.RemoveAll); err != nil {
		http.Error(w, "delete mail files: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// The logger holds the log open in append mode, so truncate rather than
	// delete: its next write lands at offset zero of the same file.
	if err := emptyDir(p.Logs, func(path string) error { return os.Truncate(path, 0) }); err != nil {
		http.Error(w, "clear logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	slog.Info("library wiped", "emails", n)
	writeJSON(w, http.StatusOK, map[string]int64{"emails": n})
}

// emptyDir applies fn to every entry of dir, collecting failures so one
// locked file doesn't stop the rest from being removed.
func emptyDir(dir string, fn func(string) error) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var errs []error
	for _, e := range entries {
		if err := fn(filepath.Join(dir, e.Name())); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
