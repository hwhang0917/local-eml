package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

const driftEML = `From: sender@example.com
To: me@example.com
Subject: drifted message
Date: Mon, 27 Jul 2026 10:00:00 +0900
Message-ID: <drift@example.com>
Content-Type: text/plain

body of the drifted message
`

func newDriftServer(t *testing.T) (*Server, paths.Paths) {
	t.Helper()
	dir := t.TempDir()
	p := paths.Paths{Base: dir, EML: filepath.Join(dir, "eml"), DB: filepath.Join(dir, "db"),
		Logs: filepath.Join(dir, "logs"), Keys: filepath.Join(dir, "keys")}
	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("dirs: %v", err)
	}
	st, err := store.Open(context.Background(), "file:"+p.DBFile())
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return &Server{Store: st, Importer: &importer.Importer{Store: st, Paths: p}}, p
}

// writeBlob puts a message in the blob store without touching the database,
// named by its content hash the way the importer would have named it.
func writeBlob(t *testing.T, p paths.Paths) string {
	t.Helper()
	sum := sha256.Sum256([]byte(driftEML))
	sha := hex.EncodeToString(sum[:])
	if err := os.WriteFile(p.BlobFor(sha), []byte(driftEML), 0o600); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	return sha
}

func getEmail(t *testing.T, s *Server, sha string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/emails/"+sha, nil))
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestGetEmailReportsMissingBlob(t *testing.T) {
	s, _ := newDriftServer(t)
	sha := strings.Repeat("a", 64)
	if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{
		Email: store.Email{SHA256: sha, Subject: "dangling"},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	code, body := getEmail(t, s, sha)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["blob_missing"] != true {
		t.Errorf("blob_missing not reported: %v", body)
	}
	if body["subject"] != "dangling" {
		t.Errorf("metadata not returned: %v", body)
	}
}

func TestGetEmailReportsUnindexedBlob(t *testing.T) {
	s, p := newDriftServer(t)
	sha := writeBlob(t, p)

	code, body := getEmail(t, s, sha)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if body["not_indexed"] != true {
		t.Errorf("not_indexed not reported: %v", body)
	}
	// The point of serving it at all is that the message is readable.
	if body["subject"] != "drifted message" {
		t.Errorf("subject not parsed from disk: %v", body)
	}
}

func TestGetEmailStillNotFoundWhenNeitherExists(t *testing.T) {
	s, _ := newDriftServer(t)
	if code, _ := getEmail(t, s, strings.Repeat("c", 64)); code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", code)
	}
}

func TestDeleteEmailOnlyWhenBlobIsGone(t *testing.T) {
	s, p := newDriftServer(t)
	ctx := context.Background()
	dangling := strings.Repeat("a", 64)
	if _, err := s.Store.InsertEmail(ctx, store.EmailRow{Email: store.Email{SHA256: dangling}}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	present := writeBlob(t, p)
	if _, err := s.Store.InsertEmail(ctx, store.EmailRow{Email: store.Email{SHA256: present}}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodDelete, "/api/emails/"+present, nil))
	if rec.Code != http.StatusConflict {
		t.Errorf("deleting a message that still has its file = %d, want 409", rec.Code)
	}

	rec = httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodDelete, "/api/emails/"+dangling, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete dangling = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if code, _ := getEmail(t, s, dangling); code != http.StatusNotFound {
		t.Errorf("row still present after delete: %d", code)
	}
}

func TestIndexOrphanedBlob(t *testing.T) {
	s, p := newDriftServer(t)
	sha := writeBlob(t, p)

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodPost, "/api/emails/"+sha+"/index", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	// Indexing must leave the blob where it was and make the row real.
	if _, err := os.Stat(p.BlobFor(sha)); err != nil {
		t.Errorf("blob disturbed by indexing: %v", err)
	}
	got, _, err := s.Store.ListEmails(context.Background(), store.ListOptions{Query: "drifted"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("indexed message not searchable: %d hits", len(got))
	}
	_, body := getEmail(t, s, sha)
	if body["not_indexed"] == true || body["blob_missing"] == true {
		t.Errorf("still reported as drifted after indexing: %v", body)
	}
}

// A blob whose name is not its content hash would otherwise be indexed under
// its real hash, silently leaving this file behind as a second copy.
func TestIndexRefusesMisnamedBlob(t *testing.T) {
	s, p := newDriftServer(t)
	sha := strings.Repeat("d", 64)
	if err := os.WriteFile(p.BlobFor(sha), []byte(driftEML), 0o600); err != nil {
		t.Fatalf("write blob: %v", err)
	}

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodPost, "/api/emails/"+sha+"/index", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	real := sha256.Sum256([]byte(driftEML))
	if _, err := os.Stat(p.BlobFor(hex.EncodeToString(real[:]))); err == nil {
		t.Error("a second copy of the blob was created")
	}
}
