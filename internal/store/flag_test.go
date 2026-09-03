package store

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEmailFlagHidesFromListings(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	bad := strings.Repeat("a", 64)
	good := strings.Repeat("b", 64)
	for _, sha := range []string{bad, good} {
		if _, err := st.InsertEmail(ctx, EmailRow{Email: Email{SHA256: sha, Subject: "s " + sha[:1], ThreadID: "t1"}}); err != nil {
			t.Fatal(err)
		}
	}

	if err := st.SetEmailFlag(ctx, bad, "junk"); !errors.Is(err, ErrInvalidFlag) {
		t.Fatalf("invalid flag: err = %v", err)
	}
	if err := st.SetEmailFlag(ctx, strings.Repeat("c", 64), FlagSpam); !errors.Is(err, ErrEmailNotFound) {
		t.Fatalf("unknown sha: err = %v", err)
	}
	if err := st.SetEmailFlag(ctx, bad, FlagPhishing); err != nil {
		t.Fatal(err)
	}

	list, total, err := st.ListEmails(ctx, ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(list) != 1 || list[0].SHA256 != good {
		t.Fatalf("library must hide flagged mail: total=%d list=%v", total, list)
	}
	thread, err := st.ListThread(ctx, "t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(thread) != 1 || thread[0].SHA256 != good {
		t.Fatalf("thread listing must hide flagged mail: %v", thread)
	}

	flagged, total, err := st.ListEmails(ctx, ListOptions{Limit: 10, FlaggedOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(flagged) != 1 || flagged[0].SHA256 != bad || flagged[0].Flag != FlagPhishing {
		t.Fatalf("flagged-only listing wrong: total=%d list=%+v", total, flagged)
	}
	if e, err := st.GetEmailBySHA(ctx, bad); err != nil || e.Flag != FlagPhishing {
		t.Fatalf("GetEmailBySHA flag = %q, err = %v", e.Flag, err)
	}

	if err := st.SetEmailFlag(ctx, bad, ""); err != nil {
		t.Fatal(err)
	}
	if _, total, err := st.ListEmails(ctx, ListOptions{Limit: 10}); err != nil || total != 2 {
		t.Fatalf("after unflag total = %d, err = %v", total, err)
	}
}
