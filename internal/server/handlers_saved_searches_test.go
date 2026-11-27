package server

import (
	"net/http"
	"strings"
	"testing"
)

func TestSavedSearchesRoundTrip(t *testing.T) {
	s, _ := newDriftServer(t)

	if rec := doJSON(t, s, http.MethodGet, "/api/saved-searches", ""); rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s", rec.Code, rec.Body)
	}
	body := `[{"name":" Receipts ","query":"receipt filename:pdf"},{"name":"Work","query":"category:blue"}]`
	if rec := doJSON(t, s, http.MethodPut, "/api/saved-searches", body); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	rec := doJSON(t, s, http.MethodGet, "/api/saved-searches", "")
	want := `[{"name":"Receipts","query":"receipt filename:pdf"},{"name":"Work","query":"category:blue"}]`
	if strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("get = %s, want %s", rec.Body, want)
	}
}

func TestSavedSearchesRejectBadLists(t *testing.T) {
	s, _ := newDriftServer(t)
	for name, body := range map[string]string{
		"no name":   `[{"name":"","query":"x"}]`,
		"no query":  `[{"name":"x","query":"  "}]`,
		"duplicate": `[{"name":"A","query":"x"},{"name":"a","query":"y"}]`,
		"long name": `[{"name":"` + strings.Repeat("n", 81) + `","query":"x"}]`,
	} {
		if rec := doJSON(t, s, http.MethodPut, "/api/saved-searches", body); rec.Code < 400 {
			t.Errorf("%s: got %d", name, rec.Code)
		}
	}
}
