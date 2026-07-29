package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

func runResync(t *testing.T, s *Server) (total int, dup int) {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodPost, "/api/imports/resync", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("resync: status %d body %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		ImportID string `json:"import_id"`
		Total    int    `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		imp, err := s.Store.GetImport(context.Background(), resp.ImportID)
		if err != nil {
			t.Fatalf("get import: %v", err)
		}
		if imp.Status == "done" || imp.Status == "error" {
			if imp.Status != "done" {
				t.Fatalf("resync job failed: %+v", imp)
			}
			return resp.Total, imp.Duplicates
		}
		if time.Now().After(deadline) {
			t.Fatalf("resync job never finished: %+v", imp)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestResyncIndexesOrphanBlobs(t *testing.T) {
	s, p := newDriftServer(t)
	s.Hub = importer.NewHub()
	s.Canceller = importer.NewCanceller()

	sha := writeBlob(t, p)

	total, _ := runResync(t, s)
	if total != 1 {
		t.Fatalf("expected 1 scanned file, got %d", total)
	}
	if _, err := s.Store.GetEmailBySHA(context.Background(), sha); err != nil {
		t.Fatalf("orphan blob not indexed after resync: %v", err)
	}

	// Second run must skip the now-indexed message and add nothing.
	if _, dup := runResync(t, s); dup != 1 {
		t.Fatalf("expected 1 duplicate on rerun, got %d", dup)
	}
	emails, total2, err := s.Store.ListEmails(context.Background(), store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(emails) != 1 || total2 != 1 {
		t.Fatalf("expected exactly 1 email after two resyncs, got %d (total %d)", len(emails), total2)
	}
}
