package store

import (
	"context"
	"testing"
	"time"
)

func TestDuplicatesGroupByMessageIDAndDate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	day := time.Date(2023, 5, 1, 9, 0, 0, 0, time.UTC)
	rows := []Email{
		// One email three times: Takeout, IMAP (different brackets/case), PST.
		{SHA256: "takeout", MessageID: "<ABC@mail.example>", SentAt: day, SizeBytes: 300},
		{SHA256: "imap", MessageID: "abc@mail.example", SentAt: day, SizeBytes: 200},
		{SHA256: "pst", MessageID: " <abc@mail.example> ", SentAt: day, SizeBytes: 100},
		// A mailer that reused the id for a different message: different date.
		{SHA256: "reused", MessageID: "<abc@mail.example>", SentAt: day.Add(48 * time.Hour)},
		// Twins without an id never group.
		{SHA256: "noid1", SentAt: day},
		{SHA256: "noid2", SentAt: day},
		// A spam copy stays out.
		{SHA256: "spamcopy", MessageID: "<abc@mail.example>", SentAt: day},
	}
	for _, e := range rows {
		if _, err := s.InsertEmail(ctx, EmailRow{Email: e}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetEmailFlag(ctx, "spamcopy", "spam"); err != nil {
		t.Fatal(err)
	}
	// What the person did to the copies must survive the merge.
	if err := s.SetEmailStarred(ctx, "pst", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEmailRead(ctx, "imap", true); err != nil {
		t.Fatal(err)
	}
	cats, _ := s.ListCategories(ctx)
	if err := s.SetEmailCategory(ctx, "pst", &cats[2].ID); err != nil {
		t.Fatal(err)
	}

	sum, err := s.DuplicateSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Groups != 1 || sum.Extra != 2 || sum.ExtraBytes != 300 {
		t.Fatalf("summary %+v, want 1 group, 2 extra copies, 300 bytes", sum)
	}
	groups, err := s.ListDuplicates(ctx, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0].Items) != 3 || groups[0].Keep != "takeout" {
		t.Fatalf("groups %+v", groups)
	}

	// A keep that isn't in the group skips it rather than guessing.
	if removed, err := s.MergeDuplicates(ctx, []MergeRequest{{Group: groups[0].Key, Keep: "reused"}}); err != nil || len(removed) != 0 {
		t.Fatalf("foreign keep: removed %v, %v", removed, err)
	}
	removed, err := s.MergeDuplicates(ctx, []MergeRequest{{Group: groups[0].Key, Keep: "imap"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Fatalf("removed %v, want takeout and pst", removed)
	}
	kept, err := s.GetEmailBySHA(ctx, "imap")
	if err != nil {
		t.Fatal(err)
	}
	if !kept.Starred || !kept.Read || kept.CategoryID == nil || *kept.CategoryID != cats[2].ID {
		t.Errorf("kept copy lost state: starred=%v read=%v category=%v", kept.Starred, kept.Read, kept.CategoryID)
	}
	for _, sha := range []string{"reused", "noid1", "noid2", "spamcopy"} {
		if ok, _ := s.EmailExists(ctx, sha); !ok {
			t.Errorf("%s was removed", sha)
		}
	}
	if sum, _ := s.DuplicateSummary(ctx); sum.Groups != 0 {
		t.Errorf("after merge: %+v", sum)
	}
}
