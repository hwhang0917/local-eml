package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSyncIntervalRoundTrip(t *testing.T) {
	s, _ := newDriftServer(t)

	put := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := newLoopbackRequest(http.MethodPut, "/api/imap/sync-interval", strings.NewReader(body))
		s.Router().ServeHTTP(rec, req)
		return rec
	}

	if rec := put(`{"seconds": 300}`); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/imap/sync-interval", nil))
	var got syncIntervalPayload
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Seconds != 300 {
		t.Fatalf("seconds = %d, want 300", got.Seconds)
	}
	if v, _ := s.Store.GetSetting(t.Context(), SyncIntervalSettingKey); v != "300" {
		t.Fatalf("stored setting = %q, want 300", v)
	}

	// 0 pauses, anything between 1 and 59 is rejected.
	if rec := put(`{"seconds": 0}`); rec.Code != http.StatusOK {
		t.Fatalf("put 0: %d %s", rec.Code, rec.Body)
	}
	if rec := put(`{"seconds": 30}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("put 30: %d, want 400", rec.Code)
	}
}
