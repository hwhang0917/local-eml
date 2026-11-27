package server

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-mbox"

	"github.com/hwhang0917/local-eml/internal/exporter"
	"github.com/hwhang0917/local-eml/internal/store"
)

func seedExport(t *testing.T) (*Server, func(string) string) {
	t.Helper()
	s, p := newDriftServer(t)
	s.Exporter = &exporter.Exporter{Store: s.Store, Paths: p}
	sha := func(c string) string { return strings.Repeat(c, 64) }
	day := func(d int) time.Time { return time.Date(2021, 3, d, 12, 0, 0, 0, time.UTC) }
	for _, e := range []store.Email{
		{SHA256: sha("a"), ThreadID: "root@x", Subject: "Deposit / refund?", FromAddr: "Landlord <landlord@example.com>", SentAt: day(2)},
		{SHA256: sha("b"), ThreadID: "root@x", Subject: "Re: Deposit", FromAddr: "me@example.com", SentAt: day(5)},
		{SHA256: sha("c"), Subject: "Newsletter", FromAddr: "news@example.com", SentAt: day(1)},
		{SHA256: sha("d"), Subject: "Landlord phish", FromAddr: "landlord@evil.example", SentAt: day(3), Flag: "phishing"},
	} {
		if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: e}); err != nil {
			t.Fatal(err)
		}
		if e.Flag != "" {
			if err := s.Store.SetEmailFlag(context.Background(), e.SHA256, e.Flag); err != nil {
				t.Fatal(err)
			}
		}
		writeBlobAs(t, p, e.SHA256, "Subject: "+e.Subject+"\r\n\r\nFrom the start\r\n")
	}
	return s, sha
}

func postExport(t *testing.T, s *Server, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := newLoopbackRequest(http.MethodPost, "/api/exports/selection", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Router().ServeHTTP(rec, req)
	return rec
}

func zipNames(t *testing.T, body []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range zr.File {
		out = append(out, f.Name)
	}
	return out
}

func TestExportSelectionZipWidensThreadsAndAddsIndex(t *testing.T) {
	s, sha := seedExport(t)
	rec := postExport(t, s, url.Values{"format": {"zip"}, "sha": {sha("b")}, "threads": {"1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "local-eml-2-messages-") {
		t.Errorf("Content-Disposition %q", cd)
	}
	names := zipNames(t, rec.Body.Bytes())
	want := []string{
		"2021-03-02_aaaaaaaa_Deposit _ refund_.eml",
		"2021-03-05_bbbbbbbb_Re_ Deposit.eml",
		exporter.IndexName,
	}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("entries %q, want %q", names, want)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(exporter.DBObjectName)) {
		t.Error("a selection export must not carry the database")
	}
}

func TestExportSearchMatchesSkipQuarantined(t *testing.T) {
	s, _ := seedExport(t)
	rec := postExport(t, s, url.Values{"format": {"mbox"}, "q": {"landlord"}, "flagged": {"1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	mr := mbox.NewReader(bytes.NewReader(rec.Body.Bytes()))
	var subjects []string
	for {
		m, err := mr.NextMessage()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(m)
		if !strings.Contains(strings.ReplaceAll(string(b), "\r", ""), "\nFrom the start") {
			t.Errorf("body line starting with From lost: %q", b)
		}
		subjects = append(subjects, strings.TrimSpace(strings.SplitN(string(b), "\n", 2)[0]))
	}
	if len(subjects) != 1 || subjects[0] != "Subject: Deposit / refund?" {
		t.Fatalf("subjects %q, want only the clean landlord mail", subjects)
	}
	if !strings.HasPrefix(rec.Body.String(), "From landlord@example.com ") {
		t.Errorf("envelope line %q", strings.SplitN(rec.Body.String(), "\n", 2)[0])
	}
}

func TestExportSelectionRejectsBadInput(t *testing.T) {
	s, sha := seedExport(t)
	for name, form := range map[string]url.Values{
		"format":  {"format": {"pdf"}, "sha": {sha("a")}},
		"sha":     {"format": {"zip"}, "sha": {"../etc"}},
		"nothing": {"format": {"zip"}, "q": {"no-such-word"}},
		"flagged": {"format": {"zip"}, "sha": {sha("d")}},
	} {
		if rec := postExport(t, s, form); rec.Code == http.StatusOK {
			t.Errorf("%s: got 200", name)
		}
	}
}
