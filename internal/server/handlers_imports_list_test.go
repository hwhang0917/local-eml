package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestListFinishedImportsHandler(t *testing.T) {
	s := newTestServer(t)
	r := s.Router()
	ctx := context.Background()

	if err := s.Store.CreateImport(ctx, store.Import{ID: "sync1", SourceKind: kindIMAPSync, Status: store.ImportQueued}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.IncImportCounters(ctx, "sync1", 3, 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.UpdateImportStatus(ctx, "sync1", store.ImportDone); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.CreateImport(ctx, store.Import{ID: "manual", SourceKind: kindIMAP, Status: store.ImportQueued}); err != nil {
		t.Fatal(err)
	}
	if err := s.Store.UpdateImportStatus(ctx, "manual", store.ImportDone); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/imports?kind=imap-sync&since=0", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	var got []store.Import
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0].ID != "sync1" || got[0].Processed != 3 || got[0].Duplicates != 1 {
		t.Fatalf("unexpected list: %+v", got)
	}

	for _, target := range []string{
		"/api/imports?since=0",
		"/api/imports?kind=imap-sync",
		"/api/imports?kind=imap-sync&since=-1",
		"/api/imports?kind=imap-sync&since=soon",
	} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newLoopbackRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", target, rec.Code)
		}
	}
}
