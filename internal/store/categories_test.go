package store

import (
	"context"
	"errors"
	"testing"
)

func mustEmail(t *testing.T, s *Store, sha, subject string) {
	t.Helper()
	if _, err := s.InsertEmail(context.Background(), EmailRow{
		Email: Email{SHA256: sha, Subject: subject},
	}); err != nil {
		t.Fatalf("insert %s: %v", sha, err)
	}
}

// The palette is the whole set: one row per colour, in palette order, present
// from the first Open without anyone creating them.
func TestCategoriesSeededOnePerColour(t *testing.T) {
	s := newTestStore(t)

	list, err := s.ListCategories(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != len(CategoryColors) {
		t.Fatalf("got %d categories, want one per palette colour (%d)", len(list), len(CategoryColors))
	}
	for i, c := range list {
		if c.Color != CategoryColors[i] {
			t.Errorf("position %d = %q, want %q", i, c.Color, CategoryColors[i])
		}
		if c.Name != "" {
			t.Errorf("%s seeded with name %q, want empty so the UI can localize it", c.Color, c.Name)
		}
	}
}

// migrate runs on every Open, so seeding must not duplicate rows or undo a
// rename the user already made.
func TestCategorySeedIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	before, _ := s.ListCategories(ctx)
	if _, err := s.RenameCategory(ctx, before[0].ID, "Work"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	if err := s.seedCategories(ctx); err != nil {
		t.Fatalf("re-seed: %v", err)
	}

	after, _ := s.ListCategories(ctx)
	if len(after) != len(before) {
		t.Fatalf("re-seed changed the row count: %d -> %d", len(before), len(after))
	}
	if after[0].Name != "Work" {
		t.Errorf("re-seed clobbered the name: %q", after[0].Name)
	}
}

func TestRenameCategory(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	list, _ := s.ListCategories(ctx)
	blue := list[4]

	renamed, err := s.RenameCategory(ctx, blue.ID, "Work")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if renamed.Name != "Work" || renamed.Color != blue.Color || renamed.ID != blue.ID {
		t.Fatalf("unexpected result: %+v", renamed)
	}

	// Clearing the name is how a user gets the colour's own name back.
	cleared, err := s.RenameCategory(ctx, blue.ID, "")
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if cleared.Name != "" {
		t.Errorf("name not cleared: %q", cleared.Name)
	}

	if _, err := s.RenameCategory(ctx, blue.ID+9999, "X"); !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("unknown id = %v, want ErrCategoryNotFound", err)
	}
}

func TestSetEmailCategoryErrors(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	list, _ := s.ListCategories(ctx)
	mustEmail(t, s, "aa", "x")

	missing := list[len(list)-1].ID + 9999
	if err := s.SetEmailCategory(ctx, "aa", &missing); !errors.Is(err, ErrCategoryNotFound) {
		t.Errorf("unknown category = %v, want ErrCategoryNotFound", err)
	}
	if err := s.SetEmailCategory(ctx, "nosuchsha", &list[0].ID); !errors.Is(err, ErrEmailNotFound) {
		t.Errorf("unknown email = %v, want ErrEmailNotFound", err)
	}
	if err := s.SetEmailCategory(ctx, "aa", nil); err != nil {
		t.Errorf("clear: %v", err)
	}
}

// total must track the filter as well as the rows — it comes from a separate
// COUNT(*) sharing the same WHERE and args slice.
func TestListEmailsCategoryFilter(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	list, _ := s.ListCategories(ctx)
	red, blue := list[0], list[4]

	mustEmail(t, s, "aa", "one")
	mustEmail(t, s, "bb", "two")
	mustEmail(t, s, "cc", "three")
	if err := s.SetEmailCategory(ctx, "aa", &red.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEmailCategory(ctx, "bb", &blue.ID); err != nil {
		t.Fatal(err)
	}

	got, total, err := s.ListEmails(ctx, ListOptions{CategoryID: &red.ID})
	if err != nil {
		t.Fatalf("by category: %v", err)
	}
	if len(got) != 1 || total != 1 || got[0].SHA256 != "aa" {
		t.Errorf("by category: %d rows total=%d, want the one red email", len(got), total)
	}

	got, total, err = s.ListEmails(ctx, ListOptions{Uncategorized: true})
	if err != nil {
		t.Fatalf("uncategorized: %v", err)
	}
	if len(got) != 1 || total != 1 || got[0].SHA256 != "cc" {
		t.Errorf("uncategorized: %d rows total=%d, want the one unassigned email", len(got), total)
	}

	if _, total, err = s.ListEmails(ctx, ListOptions{}); err != nil {
		t.Fatal(err)
	} else if total != 3 {
		t.Errorf("unfiltered total = %d, want 3", total)
	}
}
