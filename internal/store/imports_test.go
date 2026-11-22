package store

import (
	"context"
	"testing"
)

func TestListFinishedImports_FiltersKindStatusAndTime(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	mk := func(id, kind, status string, finishedAt int64) {
		t.Helper()
		if err := s.CreateImport(ctx, Import{ID: id, SourceKind: kind, SourceName: id, Status: ImportQueued}); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
		if status == ImportQueued {
			return
		}
		if _, err := s.DB.ExecContext(ctx,
			`UPDATE imports SET status = ?, finished_at = ? WHERE id = ?`, status, finishedAt, id); err != nil {
			t.Fatalf("finish %s: %v", id, err)
		}
	}
	mk("old", "imap-sync", ImportDone, 100)
	mk("b", "imap-sync", ImportDone, 300)
	mk("a", "imap-sync", ImportError, 200)
	mk("other-kind", "zip", ImportDone, 250)
	mk("running", "imap-sync", ImportQueued, 0)

	got, err := s.ListFinishedImports(ctx, "imap-sync", 200, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("want [a b] oldest first, got %+v", got)
	}
	if got[1].FinishedAt.Unix() != 300 {
		t.Fatalf("finished_at not carried: %v", got[1].FinishedAt)
	}

	got, err = s.ListFinishedImports(ctx, "imap-sync", 0, 1)
	if err != nil {
		t.Fatalf("list limited: %v", err)
	}
	if len(got) != 1 || got[0].ID != "old" {
		t.Fatalf("limit 1 from 0: want [old], got %+v", got)
	}
}
