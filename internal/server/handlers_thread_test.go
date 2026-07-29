package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestEmailThread(t *testing.T) {
	s, _ := newDriftServer(t)
	ctx := context.Background()

	rows := []store.EmailRow{
		{Email: store.Email{SHA256: strings.Repeat("a", 64), Subject: "kickoff", ThreadID: "root@x"}},
		{Email: store.Email{SHA256: strings.Repeat("b", 64), Subject: "Re: kickoff", ThreadID: "root@x"}},
		{Email: store.Email{SHA256: strings.Repeat("c", 64), Subject: "loner"}},
	}
	for _, r := range rows {
		if _, err := s.Store.InsertEmail(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	get := func(sha string) []map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/emails/"+sha+"/thread", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body.Items
	}

	if items := get(strings.Repeat("b", 64)); len(items) != 2 {
		t.Fatalf("threaded message: %d items, want 2", len(items))
	}
	if items := get(strings.Repeat("c", 64)); len(items) != 1 || items[0]["subject"] != "loner" {
		t.Fatalf("thread-less message should be a conversation of one: %+v", items)
	}
}
