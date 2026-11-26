package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/prefs"
	"github.com/hwhang0917/local-eml/internal/store"
	"github.com/hwhang0917/local-eml/internal/tempsession"
)

func doJSON(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := newLoopbackRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

func sessionMode(t *testing.T, s *Server) map[string]any {
	t.Helper()
	rec := doJSON(t, s, http.MethodGet, "/api/session", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/session: %d %s", rec.Code, rec.Body)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSessionModeAndTemporaryLaunches(t *testing.T) {
	s, p := newDriftServer(t)
	s.PrefsFile = filepath.Join(p.Base, "prefs.json")

	// A service never closes, so temporary launches can't be switched on there.
	s.RunningInteractive = false
	if got := sessionMode(t, s)["mode"]; got != ModeService {
		t.Fatalf("mode = %v, want %s", got, ModeService)
	}
	if rec := doJSON(t, s, http.MethodPut, "/api/session/temporary-launches", `{"enabled":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("service must refuse the toggle: %d %s", rec.Code, rec.Body)
	}

	s.RunningInteractive = true
	if got := sessionMode(t, s)["mode"]; got != ModeNormal {
		t.Fatalf("mode = %v, want %s", got, ModeNormal)
	}
	if rec := doJSON(t, s, http.MethodPut, "/api/session/temporary-launches", `{"enabled":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("toggle on: %d %s", rec.Code, rec.Body)
	}
	if pr, err := prefs.Load(s.PrefsFile); err != nil || !pr.TemporaryLaunches {
		t.Fatalf("prefs file not written: %+v, %v", pr, err)
	}
	if got := sessionMode(t, s)["temporary_launches"]; got != true {
		t.Fatalf("temporary_launches = %v, want true", got)
	}

	s.TemporarySession = true
	if got := sessionMode(t, s)["mode"]; got != ModeTemporary {
		t.Fatalf("mode = %v, want %s", got, ModeTemporary)
	}
}

func TestWipeData(t *testing.T) {
	s, p := newDriftServer(t)
	s.Canceller = importer.NewCanceller()
	ctx := context.Background()

	sha := writeBlob(t, p)
	if _, err := s.Store.InsertEmail(ctx, store.EmailRow{Email: store.Email{SHA256: sha}}); err != nil {
		t.Fatal(err)
	}
	logFile := p.LogFile()
	if err := os.WriteFile(logFile, []byte("subject=secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	oldLog := filepath.Join(p.Logs, "local-eml-2026-11-24.log")
	if err := os.WriteFile(oldLog, []byte("subject=older secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	keyFile := p.KeyFile()
	if err := os.WriteFile(keyFile, []byte("key"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A running job could write mail back mid-wipe; refuse until it's done.
	_, release := s.Canceller.Register(ctx, "job1")
	if rec := doJSON(t, s, http.MethodPost, "/api/data/wipe", ""); rec.Code != http.StatusConflict {
		t.Fatalf("wipe during a job: %d %s", rec.Code, rec.Body)
	}
	release()

	rec := doJSON(t, s, http.MethodPost, "/api/data/wipe", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"emails":1`) {
		t.Fatalf("wipe: %d %s", rec.Code, rec.Body)
	}
	if entries, _ := os.ReadDir(p.EML); len(entries) != 0 {
		t.Fatalf("blobs left behind: %d", len(entries))
	}
	if info, err := os.Stat(logFile); err != nil || info.Size() != 0 {
		t.Fatalf("log must be emptied (kept open by the logger): %v", err)
	}
	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Fatalf("rotated log must be deleted: %v", err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Fatalf("key file must be kept: %v", err)
	}
}

// Turning temporary sessions off from inside one must keep that session: the
// switch alone only governs the next launch, and its window was being deleted
// on close anyway.
func TestTurningTemporaryOffKeepsCurrentSession(t *testing.T) {
	s, p := newDriftServer(t)
	s.PrefsFile = filepath.Join(p.Base, "prefs.json")
	s.RunningInteractive = true

	// A normal session has nothing to keep.
	if rec := doJSON(t, s, http.MethodPut, "/api/session/keep", `{"keep":true}`); rec.Code != http.StatusConflict {
		t.Fatalf("keep outside a temporary session: %d %s", rec.Code, rec.Body)
	}

	dir, err := tempsession.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.TemporarySession = true
	s.TempSessionDir = dir
	if err := prefs.Save(s.PrefsFile, prefs.Prefs{TemporaryLaunches: true}); err != nil {
		t.Fatal(err)
	}
	if got := sessionMode(t, s)["session_kept"]; got != false {
		t.Fatalf("fresh temporary session kept = %v", got)
	}

	if rec := doJSON(t, s, http.MethodPut, "/api/session/temporary-launches", `{"enabled":false}`); rec.Code != http.StatusNoContent {
		t.Fatalf("toggle off: %d %s", rec.Code, rec.Body)
	}
	if !tempsession.IsKept(dir) || sessionMode(t, s)["session_kept"] != true {
		t.Fatal("turning temporary sessions off did not keep this one")
	}

	// The user can still let this one go.
	if rec := doJSON(t, s, http.MethodPut, "/api/session/keep", `{"keep":false}`); rec.Code != http.StatusNoContent {
		t.Fatalf("unkeep: %d %s", rec.Code, rec.Body)
	}
	if tempsession.IsKept(dir) {
		t.Fatal("keep mark not cleared")
	}
	if rec := doJSON(t, s, http.MethodPut, "/api/session/keep", `{"keep":true}`); rec.Code != http.StatusNoContent || !tempsession.IsKept(dir) {
		t.Fatalf("keep: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodPut, "/api/session/keep", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body: %d", rec.Code)
	}

	// Turning it back on means throwaway again.
	if rec := doJSON(t, s, http.MethodPut, "/api/session/temporary-launches", `{"enabled":true}`); rec.Code != http.StatusNoContent {
		t.Fatalf("toggle on: %d %s", rec.Code, rec.Body)
	}
	if tempsession.IsKept(dir) {
		t.Fatal("turning temporary sessions on left this one kept")
	}
}
