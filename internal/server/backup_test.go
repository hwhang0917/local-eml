package server

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hwhang0917/local-eml/internal/exporter"
	"github.com/hwhang0917/local-eml/internal/store"
)

func TestBackupDue(t *testing.T) {
	now := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) int64 { return now.Add(-d).Unix() }
	daily := backupConfig{Dir: "/b", Every: backupDaily}
	cases := []struct {
		name string
		cfg  backupConfig
		last backupLast
		want bool
	}{
		{"off", backupConfig{Dir: "/b", Every: backupOff}, backupLast{}, false},
		{"no folder", backupConfig{Every: backupDaily}, backupLast{}, false},
		{"never ran", daily, backupLast{}, true},
		{"ran an hour ago", daily, backupLast{At: ago(time.Hour), OK: true}, false},
		{"ran yesterday", daily, backupLast{At: ago(25 * time.Hour), OK: true}, true},
		{"weekly, 3 days", backupConfig{Dir: "/b", Every: backupWeekly}, backupLast{At: ago(72 * time.Hour), OK: true}, false},
		{"failed 10 min ago", daily, backupLast{At: ago(10 * time.Minute)}, false},
		{"failed 2 hours ago", daily, backupLast{At: ago(2 * time.Hour)}, true},
	}
	for _, c := range cases {
		if got := backupDue(c.cfg, c.last, now); got != c.want {
			t.Errorf("%s: due=%v, want %v", c.name, got, c.want)
		}
	}
}

func TestBackupSettingsAndRun(t *testing.T) {
	s, p := newDriftServer(t)
	s.Exporter = &exporter.Exporter{Store: s.Store, Paths: p}
	sha := strings.Repeat("a", 64)
	if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: store.Email{SHA256: sha}}); err != nil {
		t.Fatal(err)
	}
	writeBlobAs(t, p, sha, "x")

	for name, body := range map[string]string{
		"relative": `{"dir":"rel/dir","every":"daily"}`,
		"no dir":   `{"dir":"","every":"weekly"}`,
		"bad freq": `{"dir":"/tmp/x","every":"hourly"}`,
	} {
		if rec := doJSON(t, s, http.MethodPut, "/api/backup", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	dir := filepath.Join(t.TempDir(), "backup")
	rec := doJSON(t, s, http.MethodPut, "/api/backup", `{"dir":`+jsonString(dir)+`,"every":"daily"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"every":"daily"`) {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodPost, "/api/backup/run", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("run: %d %s", rec.Code, rec.Body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for s.backup.isRunning() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	rec = doJSON(t, s, http.MethodGet, "/api/backup", "")
	if !strings.Contains(rec.Body.String(), `"ok":true`) || !strings.Contains(rec.Body.String(), `"added":1`) {
		t.Fatalf("after run: %s", rec.Body)
	}
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}
