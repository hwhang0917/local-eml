package store

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCountEmailsByDay(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	insert := func(sha string, sent time.Time) {
		t.Helper()
		row := EmailRow{Email: Email{
			SHA256:   strings.Repeat(sha, 64),
			Filename: sha + ".eml", Subject: "s", FromAddr: "f@example",
			SentAt: sent,
		}, BodyText: "b"}
		if _, err := s.InsertEmail(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	insert("a", time.Date(2026, 8, 15, 0, 0, 0, 0, time.Local))
	insert("b", time.Date(2026, 8, 15, 23, 59, 59, 0, time.Local))
	insert("c", time.Date(2026, 7, 31, 23, 59, 59, 0, time.Local)) // outside month bounds
	insert("d", time.Time{})                                       // undated: sent_at = 0

	// Bounds computed exactly as handleStatsCalendar does.
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	end := start.AddDate(0, 1, 0).Add(-time.Second)

	got, err := s.CountEmailsByDay(ctx, start.Unix(), end.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["2026-08-15"] != 2 {
		t.Errorf("counts = %v, want {2026-08-15: 2}", got)
	}

	empty, err := s.CountEmailsByDay(ctx,
		time.Date(2030, 1, 1, 0, 0, 0, 0, time.Local).Unix(),
		time.Date(2030, 1, 31, 23, 59, 59, 0, time.Local).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("empty month = %v, want non-nil empty map", empty)
	}
}
