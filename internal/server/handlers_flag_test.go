package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

// Flagged mail must be refused by the rich-content endpoints regardless of
// what the UI asks for; plain text and metadata stay available.
func TestFlaggedEmailIsPlainTextOnly(t *testing.T) {
	s, _ := newDriftServer(t) // has Store and Importer, so blob checks work
	r := s.Router()
	sha := strings.Repeat("d", 64)
	if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: store.Email{SHA256: sha}}); err != nil {
		t.Fatal(err)
	}
	do := func(method, path, body string) *httptest.ResponseRecorder {
		var rd *strings.Reader
		if body != "" {
			rd = strings.NewReader(body)
		} else {
			rd = strings.NewReader("")
		}
		req := newLoopbackRequest(method, path, rd)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	if rec := do(http.MethodPut, "/api/emails/"+sha+"/flag", `{"flag":"bogus"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bogus flag: %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodPut, "/api/emails/"+sha+"/flag", `{"flag":"phishing"}`); rec.Code != http.StatusNoContent {
		t.Fatalf("flag: %d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{"/html", "/cid/x", "/attachments/0"} {
		if rec := do(http.MethodGet, "/api/emails/"+sha+p, ""); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s on flagged mail: want 403, got %d", p, rec.Code)
		}
	}
	if rec := do(http.MethodGet, "/api/emails/"+sha, ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"flag":"phishing"`) {
		t.Fatalf("metadata must still be served with the flag: %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodGet, "/api/emails?limit=10", ""); !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Fatalf("library must hide flagged mail: %s", rec.Body)
	}
	if rec := do(http.MethodGet, "/api/emails?flagged=1&limit=10", ""); !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("flagged listing must show it: %s", rec.Body)
	}

	if rec := do(http.MethodDelete, "/api/emails/"+sha+"/flag", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("unflag: %d %s", rec.Code, rec.Body)
	}
	if rec := do(http.MethodGet, "/api/emails/"+sha+"/html", ""); rec.Code == http.StatusForbidden {
		t.Fatalf("unflagged mail must not be refused (got 403)")
	}
}
