package store

import (
	"context"
	"testing"
	"time"
)

func TestListEmailsGroupedByThread(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	day := func(d int) time.Time { return time.Date(2026, 1, d, 12, 0, 0, 0, time.UTC) }
	rows := []EmailRow{
		{Email: Email{SHA256: "a1", Subject: "kickoff", ThreadID: "root@x", SentAt: day(1)}, BodyText: "hello"},
		{Email: Email{SHA256: "a2", Subject: "Re: kickoff", ThreadID: "root@x", SentAt: day(3)}, BodyText: "reply"},
		{Email: Email{SHA256: "b1", Subject: "loner", SentAt: day(2)}, BodyText: "alone"},
	}
	for _, r := range rows {
		if _, err := s.InsertEmail(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	got, total, err := s.ListEmails(ctx, ListOptions{GroupThreads: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(got) != 2 {
		t.Fatalf("grouped list: %d rows (total %d), want 2", len(got), total)
	}
	// Default sort is sent_at DESC; the thread's representative is its newest.
	if got[0].SHA256 != "a2" || got[0].ThreadCount != 2 {
		t.Errorf("row 0 = %s count %d, want a2 count 2", got[0].SHA256, got[0].ThreadCount)
	}
	if got[1].SHA256 != "b1" || got[1].ThreadCount != 1 {
		t.Errorf("row 1 = %s count %d, want b1 count 1", got[1].SHA256, got[1].ThreadCount)
	}

	// A search matching only the root still surfaces the conversation, counted
	// by matching members.
	got, total, err = s.ListEmails(ctx, ListOptions{GroupThreads: true, Query: "hello", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(got) != 1 || got[0].SHA256 != "a1" || got[0].ThreadCount != 1 {
		t.Fatalf("grouped search: %+v (total %d)", got, total)
	}

	// Ungrouped listing is unchanged: three rows, no thread counts.
	got, total, err = s.ListEmails(ctx, ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if total != 3 || got[0].ThreadCount != 0 {
		t.Fatalf("flat list changed: total %d, count %d", total, got[0].ThreadCount)
	}
}
