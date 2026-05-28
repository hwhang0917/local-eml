package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dsn := "file:" + filepath.Join(dir, "test.db") + "?_pragma=journal_mode(WAL)"
	s, err := Open(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func ptrInt(v int) *int       { return &v }
func ptrStr(v string) *string { return &v }

func TestIMAPProfiles_UpsertCreatesAndList(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p, err := s.UpsertIMAPProfile(ctx, IMAPProfile{
		Name: "Work", Host: "imap.example.com", Port: ptrInt(993),
		Username: "user@example.com", Folder: ptrStr("INBOX"),
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if p.ID == 0 || p.CreatedAt == 0 || p.UpdatedAt == 0 {
		t.Fatalf("expected id and timestamps set, got %+v", p)
	}

	list, err := s.ListIMAPProfiles(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Work" || list[0].Host != "imap.example.com" {
		t.Fatalf("unexpected list: %+v", list)
	}
	if list[0].Port == nil || *list[0].Port != 993 {
		t.Fatalf("port not round-tripped: %+v", list[0].Port)
	}
	if list[0].Folder == nil || *list[0].Folder != "INBOX" {
		t.Fatalf("folder not round-tripped: %+v", list[0].Folder)
	}
}

func TestIMAPProfiles_UpsertUpdatesByName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first, err := s.UpsertIMAPProfile(ctx, IMAPProfile{
		Name: "Work", Host: "old.example.com", Username: "u",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := s.UpsertIMAPProfile(ctx, IMAPProfile{
		Name: "Work", Host: "new.example.com", Username: "u2", Folder: ptrStr("Archive"),
	})
	if err != nil {
		t.Fatalf("upsert update: %v", err)
	}
	if updated.ID != first.ID {
		t.Fatalf("id changed: was %d now %d", first.ID, updated.ID)
	}
	if updated.CreatedAt != first.CreatedAt {
		t.Fatalf("created_at changed: was %d now %d", first.CreatedAt, updated.CreatedAt)
	}
	if updated.UpdatedAt < first.UpdatedAt {
		t.Fatalf("updated_at regressed")
	}
	if updated.Host != "new.example.com" || updated.Username != "u2" {
		t.Fatalf("update fields not applied: %+v", updated)
	}

	list, _ := s.ListIMAPProfiles(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 row after update, got %d", len(list))
	}
}

func TestIMAPProfiles_Delete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p, _ := s.UpsertIMAPProfile(ctx, IMAPProfile{Name: "X", Host: "h", Username: "u"})
	if err := s.DeleteIMAPProfile(ctx, p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := s.DeleteIMAPProfile(ctx, p.ID); !errors.Is(err, ErrIMAPProfileNotFound) {
		t.Fatalf("expected ErrIMAPProfileNotFound, got %v", err)
	}
	list, _ := s.ListIMAPProfiles(ctx)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}
}

func TestIMAPProfiles_NullablePortAndFolder(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.UpsertIMAPProfile(ctx, IMAPProfile{Name: "Y", Host: "h", Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListIMAPProfiles(ctx)
	if list[0].Port != nil {
		t.Fatalf("expected nil port, got %v", list[0].Port)
	}
	if list[0].Folder != nil {
		t.Fatalf("expected nil folder, got %v", list[0].Folder)
	}
}
