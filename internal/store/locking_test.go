package store

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestProfileSaveWaitsForBusyWriter reproduces "database is locked" on
// profile save: another connection holds the write lock (as an import, IMAP
// sync or backfill does) and the save has to wait for it, not fail at once.
func TestProfileSaveWaitsForBusyWriter(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	// Pin the connection that ran Open's setup so the save is forced onto a
	// fresh pool connection, the case that broke.
	holder, err := s.DB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if _, err := holder.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	if _, err := holder.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES ('busy', '1')`); err != nil {
		t.Fatal(err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(300 * time.Millisecond)
		_, _ = holder.ExecContext(ctx, "COMMIT")
		close(released)
	}()

	if _, err := s.UpsertS3Profile(ctx, S3Profile{Name: "a", Bucket: "b"}); err != nil {
		t.Fatalf("S3 save while another write is open: %v", err)
	}
	if _, err := s.UpsertIMAPProfile(ctx, IMAPProfile{Name: "a", Host: "h", Username: "u"}); err != nil {
		t.Fatalf("IMAP save while another write is open: %v", err)
	}
	<-released
}

// TestConcurrentWritesDoNotLock runs profile saves against a stream of other
// writers. Upsert reads before it writes, so a deferred transaction would
// have to upgrade its lock mid-way, which SQLite refuses with SQLITE_BUSY
// (no retry, whatever busy_timeout says) once another writer committed.
func TestConcurrentWritesDoNotLock(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	var wg sync.WaitGroup
	errs := make(chan error, 1000)
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if _, err := s.DB.ExecContext(ctx,
					`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`,
					fmt.Sprintf("k%d", w), fmt.Sprint(i)); err != nil {
					errs <- fmt.Errorf("writer %d: %w", w, err)
				}
			}
		}(w)
	}
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				name := fmt.Sprintf("p%d", w)
				if _, err := s.UpsertS3Profile(ctx, S3Profile{Name: name, Bucket: fmt.Sprint(i)}); err != nil {
					errs <- fmt.Errorf("s3 %d: %w", w, err)
				}
				if _, err := s.UpsertIMAPProfile(ctx, IMAPProfile{Name: name, Host: "h", Username: "u"}); err != nil {
					errs <- fmt.Errorf("imap %d: %w", w, err)
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}
