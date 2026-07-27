package store

import (
	"context"
	"testing"
)

func insert(t *testing.T, s *Store, sha, subject, body string) int64 {
	t.Helper()
	id, err := s.InsertEmail(context.Background(), EmailRow{
		Email:    Email{SHA256: sha, Subject: subject, FromAddr: "a@example.com"},
		BodyText: body,
	})
	if err != nil {
		t.Fatalf("insert %s: %v", sha, err)
	}
	return id
}

func search(t *testing.T, s *Store, q string) []string {
	t.Helper()
	got, _, err := s.ListEmails(context.Background(), ListOptions{Query: q})
	if err != nil {
		t.Fatalf("search %q: %v", q, err)
	}
	out := make([]string, 0, len(got))
	for _, e := range got {
		out = append(out, e.SHA256)
	}
	return out
}

// The contentless FTS index keeps the deleted row's entry. That is only safe
// while its id is never reissued — otherwise the next import inherits the
// deleted message's search terms.
func TestDeletedEmailIDIsNeverReused(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	insert(t, s, "aa", "keeper", "keeper body")
	newest := insert(t, s, "bb", "doomed", "quarterly platypus report")

	if err := s.DeleteEmailBySHA(ctx, "bb"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if hits := search(t, s, "platypus"); len(hits) != 0 {
		t.Fatalf("deleted email still searchable: %v", hits)
	}

	reused := insert(t, s, "cc", "fresh", "fresh body")
	if reused <= newest {
		t.Fatalf("id %d reuses or precedes deleted id %d", reused, newest)
	}
	// The real failure this guards: the orphaned index entry attaching itself
	// to whichever message got the recycled id.
	if hits := search(t, s, "platypus"); len(hits) != 0 {
		t.Errorf("deleted email's terms resolved to a later message: %v", hits)
	}
	if hits := search(t, s, "fresh"); len(hits) != 1 || hits[0] != "cc" {
		t.Errorf("new email not searchable: %v", hits)
	}
}

func TestDeleteEmailMissingRow(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteEmailBySHA(context.Background(), "nope"); err != ErrEmailNotFound {
		t.Fatalf("got %v, want ErrEmailNotFound", err)
	}
}
