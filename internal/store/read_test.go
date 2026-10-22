package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEmailReadState(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	a := strings.Repeat("a", 64)
	b := strings.Repeat("b", 64)
	for _, sha := range []string{a, b} {
		if _, err := st.InsertEmail(ctx, EmailRow{Email: Email{SHA256: sha, Subject: "s"}}); err != nil {
			t.Fatal(err)
		}
	}

	// New rows start unread so a freshly imported message stands out.
	unread, total, err := st.ListEmails(ctx, ListOptions{Limit: 10, UnreadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(unread) != 2 || unread[0].Read {
		t.Fatalf("new rows must be unread: total=%d rows=%+v", total, unread)
	}

	if err := st.SetEmailRead(ctx, strings.Repeat("c", 64), true); !errors.Is(err, ErrEmailNotFound) {
		t.Fatalf("unknown sha: err=%v, want ErrEmailNotFound", err)
	}
	if err := st.SetEmailRead(ctx, a, true); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetEmailBySHA(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Read {
		t.Fatal("read flag not persisted")
	}
	_, total, err = st.ListEmails(ctx, ListOptions{Limit: 10, UnreadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("unread total=%d, want 1", total)
	}

	// The first-time choice flips every row at once, in either direction.
	if err := st.SetAllEmailsRead(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, total, _ = st.ListEmails(ctx, ListOptions{Limit: 10, UnreadOnly: true}); total != 0 {
		t.Fatalf("after mark all read: unread total=%d, want 0", total)
	}
	if err := st.SetAllEmailsRead(ctx, false); err != nil {
		t.Fatal(err)
	}
	if _, total, _ = st.ListEmails(ctx, ListOptions{Limit: 10, UnreadOnly: true}); total != 2 {
		t.Fatalf("after mark all unread: unread total=%d, want 2", total)
	}
}
