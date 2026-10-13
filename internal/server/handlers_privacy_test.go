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
	logFile := filepath.Join(p.Logs, "local-eml.log")
	if err := os.WriteFile(logFile, []byte("subject=secret\n"), 0o600); err != nil {
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
	if _, err := os.Stat(keyFile); err != nil {
		t.Fatalf("key file must be kept: %v", err)
	}
}
