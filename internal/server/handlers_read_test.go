package server

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestReadTrackingFirstTimeChoice(t *testing.T) {
	s, _ := newDriftServer(t)
	ctx := context.Background()
	for _, c := range []string{"a", "b"} {
		if _, err := s.Store.InsertEmail(ctx, store.EmailRow{Email: store.Email{SHA256: strings.Repeat(c, 64)}}); err != nil {
			t.Fatal(err)
		}
	}

	// A library that never heard of the feature reports it off and unconfigured,
	// which is what tells the SPA to ask the first-time question.
	rec := doJSON(t, s, http.MethodGet, "/api/read-tracking", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":false`) ||
		!strings.Contains(rec.Body.String(), `"configured":false`) {
		t.Fatalf("initial state: %d %s", rec.Code, rec.Body)
	}

	if rec := doJSON(t, s, http.MethodPut, "/api/read-tracking", `{"enabled":true,"existing":"later"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("bad existing value: %d %s", rec.Code, rec.Body)
	}

	rec = doJSON(t, s, http.MethodPut, "/api/read-tracking", `{"enabled":true,"existing":"read"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"enabled":true`) ||
		!strings.Contains(rec.Body.String(), `"configured":true`) {
		t.Fatalf("enable: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodGet, "/api/emails?unread=1&limit=10", ""); !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Fatalf("existing mail must be read: %s", rec.Body)
	}

	// Turning it off and on again keeps the state; only the first time asks.
	if rec := doJSON(t, s, http.MethodPut, "/api/read-tracking", `{"enabled":false}`); rec.Code != http.StatusOK {
		t.Fatalf("disable: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodPut, "/api/read-tracking", `{"enabled":true}`); rec.Code != http.StatusOK {
		t.Fatalf("re-enable: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodGet, "/api/emails?unread=1&limit=10", ""); !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Fatalf("re-enable must not reset state: %s", rec.Body)
	}
}

func TestReadEmailToggle(t *testing.T) {
	s, _ := newDriftServer(t)
	sha := strings.Repeat("d", 64)
	if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: store.Email{SHA256: sha}}); err != nil {
		t.Fatal(err)
	}

	if rec := doJSON(t, s, http.MethodPut, "/api/emails/"+sha+"/read", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("mark read: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodGet, "/api/emails/"+sha, ""); !strings.Contains(rec.Body.String(), `"read":true`) {
		t.Fatalf("read not reported: %s", rec.Body)
	}
	if rec := doJSON(t, s, http.MethodDelete, "/api/emails/"+sha+"/read", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("mark unread: %d %s", rec.Code, rec.Body)
	}
	if rec := doJSON(t, s, http.MethodGet, "/api/emails?unread=1&limit=10", ""); !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("unread filter: %s", rec.Body)
	}
	if rec := doJSON(t, s, http.MethodPut, "/api/emails/"+strings.Repeat("e", 64)+"/read", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown sha: %d", rec.Code)
	}
}
