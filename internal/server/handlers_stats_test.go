package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestStats(t *testing.T) {
	s, _ := newDriftServer(t)
	ctx := context.Background()

	rows := []store.EmailRow{
		// Sunday 00:30 UTC: 09:30 Sunday in Seoul.
		{Email: store.Email{SHA256: strings.Repeat("a", 64), FromAddr: "Alice <Alice@Example.com>",
			SentAt: time.Date(2020, 3, 1, 0, 30, 0, 0, time.UTC), SizeBytes: 100, Starred: true}},
		// Saturday 23:30 UTC: 08:30 Sunday, June 2, in Seoul.
		{Email: store.Email{SHA256: strings.Repeat("b", 64), FromAddr: "alice@example.com",
			SentAt: time.Date(2024, 6, 1, 23, 30, 0, 0, time.UTC), SizeBytes: 250,
			HasAttachments: true, AttachmentCount: 2},
			Attachments: []store.AttachmentMeta{
				{Index: 0, Filename: "a.pdf", ContentType: "application/pdf", Size: 100},
				{Index: 1, Filename: "b.png", ContentType: "image/png", Size: 50},
			}},
		{Email: store.Email{SHA256: strings.Repeat("c", 64), FromAddr: "bob@other.org"}},
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
	if err := s.Store.SetEmailFlag(ctx, strings.Repeat("c", 64), store.FlagSpam); err != nil {
		t.Fatalf("flag: %v", err)
	}

	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/stats?tz_offset=540", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var st store.Stats
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if st.TotalCount != 3 || st.TotalBytes != 350 || st.StarredCount != 1 ||
		st.AttachmentCount != 1 || st.UndatedCount != 1 || st.SpamCount != 1 ||
		st.UncategorizedCount != 3 || st.PhishingCount != 0 {
		t.Errorf("scalars wrong: %+v", st)
	}
	want := []store.YearCount{{Year: "2020", Count: 1}, {Year: "2024", Count: 1}}
	if len(st.PerYear) != 2 || st.PerYear[0] != want[0] || st.PerYear[1] != want[1] {
		t.Errorf("per_year = %+v, want %+v", st.PerYear, want)
	}
	wantMonths := []store.MonthCount{{Month: "2020-03", Count: 1, Bytes: 100}, {Month: "2024-06", Count: 1, Bytes: 250}}
	if len(st.PerMonth) != 2 || st.PerMonth[0] != wantMonths[0] || st.PerMonth[1] != wantMonths[1] {
		t.Errorf("per_month = %+v, want %+v", st.PerMonth, wantMonths)
	}
	if st.Heatmap[0][9] != 1 || st.Heatmap[0][8] != 1 {
		t.Errorf("heatmap Sunday = %v, want 1 at 08 and 09", st.Heatmap[0])
	}
	if len(st.TopSenders) != 2 || st.TopSenders[0] != (store.SenderCount{Address: "alice@example.com", Name: "Alice", Count: 2}) {
		t.Errorf("top_senders = %+v", st.TopSenders)
	}
	if len(st.TopDomains) != 2 || st.TopDomains[0] != (store.DomainCount{Domain: "example.com", Count: 2}) {
		t.Errorf("top_domains = %+v", st.TopDomains)
	}
	if len(st.PerCategory) != 0 {
		t.Errorf("per_category = %+v, want empty", st.PerCategory)
	}
	if st.AttachmentBytes != 150 || len(st.PerKind) != 2 ||
		st.PerKind[0] != (store.KindUsage{Kind: store.KindPDF, Count: 1, Bytes: 100}) {
		t.Errorf("attachments = %d %+v", st.AttachmentBytes, st.PerKind)
	}
	if len(st.Largest) != 3 || st.Largest[0].SHA256 != strings.Repeat("b", 64) || st.Largest[0].SizeBytes != 250 {
		t.Errorf("largest = %+v", st.Largest)
	}
}

func TestStatsBadOffsetIsUTC(t *testing.T) {
	s, _ := newDriftServer(t)
	if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: store.Email{
		SHA256: strings.Repeat("a", 64), FromAddr: "a@x.com",
		SentAt: time.Date(2020, 3, 1, 0, 30, 0, 0, time.UTC)}}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"", "?tz_offset=junk", "?tz_offset=99999"} {
		rec := httptest.NewRecorder()
		s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/stats"+q, nil))
		var st store.Stats
		if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
			t.Fatalf("%q: %v", q, err)
		}
		if st.Heatmap[0][0] != 1 {
			t.Errorf("%q: heatmap Sunday = %v, want 1 at 00 (UTC)", q, st.Heatmap[0])
		}
	}
}

func TestAppStats(t *testing.T) {
	s, p := newDriftServer(t)
	if err := os.WriteFile(p.BlobFor(strings.Repeat("a", 64)), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, newLoopbackRequest(http.MethodGet, "/api/stats/app", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var a appStats
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if a.MemoryBytes == 0 || a.NumCPU == 0 || a.GoVersion == "" || a.CPUPercent < 0 {
		t.Errorf("process stats = %+v", a)
	}
	if a.Disk == nil || a.Disk.EMLFiles != 1 || a.Disk.EMLBytes != 5 || a.Disk.DBBytes == 0 || a.Disk.TotalBytes == 0 {
		t.Errorf("disk = %+v", a.Disk)
	}
}
