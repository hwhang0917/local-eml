package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

// seedBulk stores three messages with blobs: a and b share a conversation,
// c stands alone.
func seedBulk(t *testing.T) (*Server, func(string) string) {
	t.Helper()
	s, p := newDriftServer(t)
	sha := func(c string) string { return strings.Repeat(c, 64) }
	for _, e := range []store.Email{
		{SHA256: sha("a"), ThreadID: "root@x"},
		{SHA256: sha("b"), ThreadID: "root@x"},
		{SHA256: sha("c")},
	} {
		if _, err := s.Store.InsertEmail(context.Background(), store.EmailRow{Email: e}); err != nil {
			t.Fatal(err)
		}
		writeBlobAs(t, p, e.SHA256, "body")
	}
	return s, sha
}

func bulkBody(action string, threads bool, shas ...string) string {
	b := `{"action":"` + action + `","sha256s":["` + strings.Join(shas, `","`) + `"]`
	if threads {
		b += `,"threads":true`
	}
	return b + "}"
}

func isRead(t *testing.T, s *Server, sha string) bool {
	t.Helper()
	e, err := s.Store.GetEmailBySHA(context.Background(), sha)
	if err != nil {
		t.Fatal(err)
	}
	return e.Read
}

func TestBulkReadUnread(t *testing.T) {
	s, sha := seedBulk(t)

	rec := doJSON(t, s, http.MethodPost, "/api/emails/bulk", bulkBody("read", false, sha("a"), sha("c"), sha("f")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"affected":2`) {
		t.Fatalf("read: %d %s", rec.Code, rec.Body)
	}
	if !isRead(t, s, sha("a")) || isRead(t, s, sha("b")) || !isRead(t, s, sha("c")) {
		t.Fatal("read must touch only the listed messages")
	}

	// A grouped row stands for its whole conversation.
	rec = doJSON(t, s, http.MethodPost, "/api/emails/bulk", bulkBody("unread", true, sha("b")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"affected":2`) {
		t.Fatalf("unread thread: %d %s", rec.Code, rec.Body)
	}
	if isRead(t, s, sha("a")) || isRead(t, s, sha("b")) || !isRead(t, s, sha("c")) {
		t.Fatal("threads must widen to the conversation and no further")
	}
}

func TestBulkDeleteRemovesRowsAndBlobs(t *testing.T) {
	s, sha := seedBulk(t)

	rec := doJSON(t, s, http.MethodPost, "/api/emails/bulk", bulkBody("delete", true, sha("a")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"affected":2`) {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	for _, c := range []string{"a", "b"} {
		if ok, err := s.Store.EmailExists(context.Background(), sha(c)); ok || err != nil {
			t.Errorf("%s row survived: %v", c, err)
		}
		if _, err := os.Stat(s.Importer.Paths.BlobFor(sha(c))); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s blob survived: %v", c, err)
		}
	}
	if _, err := os.Stat(s.Importer.Paths.BlobFor(sha("c"))); err != nil {
		t.Errorf("unselected blob removed: %v", err)
	}
}

func TestBulkRejectsBadInput(t *testing.T) {
	s, sha := seedBulk(t)
	for name, body := range map[string]string{
		"unknown action": bulkBody("archive", false, sha("a")),
		"no shas":        `{"action":"read","sha256s":[]}`,
		"bad sha":        bulkBody("delete", false, "../../etc/passwd"),
	} {
		if rec := doJSON(t, s, http.MethodPost, "/api/emails/bulk", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body)
		}
	}
	if _, err := os.Stat(s.Importer.Paths.BlobFor(sha("a"))); err != nil {
		t.Errorf("rejected request touched a blob: %v", err)
	}
}
