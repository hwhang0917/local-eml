package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestListAttachmentsHandler(t *testing.T) {
	s, _ := newDriftServer(t)
	ctx := context.Background()
	for _, r := range []store.EmailRow{
		{
			Email: store.Email{SHA256: strings.Repeat("a", 64), Subject: "pics", FromAddr: "alice@example.com", AttachmentCount: 2},
			Attachments: []store.AttachmentMeta{
				{Index: 0, Filename: "a.png", ContentType: "image/png", Size: 10},
				{Index: 1, Filename: "b.pdf", ContentType: "application/pdf", Size: 30},
			},
		},
		{
			Email:       store.Email{SHA256: strings.Repeat("b", 64), Subject: "zip", FromAddr: "bob@example.com", AttachmentCount: 1},
			Attachments: []store.AttachmentMeta{{Index: 0, Filename: "c.zip", ContentType: "application/zip", Size: 20}},
		},
	} {
		if _, err := s.Store.InsertEmail(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	get := func(qs string) map[string]any {
		t.Helper()
		rec := doJSON(t, s, http.MethodGet, "/api/attachments"+qs, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s: %d %s", qs, rec.Code, rec.Body)
		}
		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		return body
	}

	body := get("?sort=size&order=desc")
	items := body["items"].([]any)
	if len(items) != 3 || body["total"].(float64) != 3 || body["total_bytes"].(float64) != 60 {
		t.Fatalf("body = %v", body)
	}
	first := items[0].(map[string]any)
	if first["filename"] != "b.pdf" || first["email_sha256"] != strings.Repeat("a", 64) ||
		first["index"].(float64) != 1 || first["kind"] != "pdf" || first["email_subject"] != "pics" {
		t.Errorf("first = %v", first)
	}

	if body := get("?kind=archive"); body["total"].(float64) != 1 {
		t.Errorf("kind=archive total = %v", body["total"])
	}
	// An unknown kind is ignored rather than blanking the list.
	if body := get("?kind=bogus"); body["total"].(float64) != 3 {
		t.Errorf("kind=bogus total = %v", body["total"])
	}
	if body := get("?sender=bob"); body["total"].(float64) != 1 {
		t.Errorf("sender=bob total = %v", body["total"])
	}
	if body := get("?q=.PNG"); body["total"].(float64) != 1 {
		t.Errorf("q=.PNG total = %v", body["total"])
	}
}
