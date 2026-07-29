package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestStats(t *testing.T) {
	s, _ := newDriftServer(t)
	ctx := context.Background()

	rows := []store.EmailRow{
		{Email: store.Email{SHA256: strings.Repeat("a", 64), FromAddr: "alice@example.com",
			SentAt: time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC), SizeBytes: 100, Starred: true}},
		{Email: store.Email{SHA256: strings.Repeat("b", 64), FromAddr: "alice@example.com",
			SentAt: time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), SizeBytes: 250,
			HasAttachments: true, AttachmentCount: 2}},
		{Email: store.Email{SHA256: strings.Repeat("c", 64), FromAddr: "bob@example.com"}},
	}
	for _, r := range rows {
		if _, err := s.Store.InsertEmail(ctx, r); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if r.Email.Starred {
			if err := s.Store.SetEmailStarred(ctx, r.Email.SHA256, true); err != nil {
				t.Fatalf("star: %v", err)
			}
		}
	}

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var st store.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if st.TotalCount != 3 || st.TotalBytes != 350 || st.StarredCount != 1 ||
		st.AttachmentCount != 1 || st.UndatedCount != 1 {
		t.Errorf("scalars wrong: %+v", st)
	}
	want := []store.YearCount{{Year: "2020", Count: 1}, {Year: "2024", Count: 1}}
	if len(st.PerYear) != 2 || st.PerYear[0] != want[0] || st.PerYear[1] != want[1] {
		t.Errorf("per_year = %+v, want %+v", st.PerYear, want)
	}
	if len(st.TopSenders) != 2 || st.TopSenders[0].From != "alice@example.com" ||
		st.TopSenders[0].Count != 2 {
		t.Errorf("top_senders = %+v", st.TopSenders)
	}
	if len(st.PerCategory) != 0 {
		t.Errorf("per_category = %+v, want empty", st.PerCategory)
	}
}
