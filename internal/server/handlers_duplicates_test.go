package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestMergeAllDuplicatesRemovesExtraBlobs(t *testing.T) {
	s, p := newDriftServer(t)
	sha := func(c string) string { return strings.Repeat(c, 64) }
	day := time.Date(2023, 5, 1, 9, 0, 0, 0, time.UTC)
	for _, e := range []store.Email{
		{SHA256: sha("a"), MessageID: "<x@y>", SentAt: day},
		{SHA256: sha("b"), MessageID: "x@y", SentAt: day},
		{SHA256: sha("c"), MessageID: "<other@y>", SentAt: day},
	} {
		if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: e}); err != nil {
			t.Fatal(err)
		}
		writeBlobAs(t, p, e.SHA256, "x")
	}

	rec := doJSON(t, s, http.MethodGet, "/api/duplicates", "")
	var list struct {
		Summary store.DuplicateSummary `json:"summary"`
		Groups  []store.DuplicateGroup `json:"groups"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil || list.Summary.Extra != 1 || len(list.Groups) != 1 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}

	if rec := doJSON(t, s, http.MethodPost, "/api/duplicates/merge", `{"groups":[{"group":"x","keep":"../x"}]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad keep: %d", rec.Code)
	}
	rec = doJSON(t, s, http.MethodPost, "/api/duplicates/merge", `{"all":true}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"removed":1`) {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(p.BlobFor(sha("b"))); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("extra copy's file survived: %v", err)
	}
	for _, c := range []string{"a", "c"} {
		if _, err := os.Stat(p.BlobFor(sha(c))); err != nil {
			t.Errorf("%s file removed: %v", c, err)
		}
	}
}
