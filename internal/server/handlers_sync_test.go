package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSyncIntervalRoundTrip(t *testing.T) {
	s, _ := newDriftServer(t)

	put := func(body string) *httptest.ResponseRecorder {
		return doJSON(t, s, http.MethodPut, "/api/imap/sync-interval", body)
	}

	if rec := put(`{"seconds": 300}`); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}

	rec := doJSON(t, s, http.MethodGet, "/api/imap/sync-interval", "")
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
