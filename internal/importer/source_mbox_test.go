package importer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

// Three messages, mboxrd-style: the body line quoted as ">From ..." must stay
// inside message two, not start a new one.
const fixtureMbox = "From alice@example.com Mon Jan  2 15:04:05 2023\r\n" +
	"From: alice@example.com\r\nTo: b@example.com\r\nSubject: first\r\n\r\nbody one\r\n" +
	"\r\n" +
	"From alice@example.com Mon Jan  2 15:05:05 2023\r\n" +
	"From: alice@example.com\r\nTo: b@example.com\r\nSubject: second\r\n\r\n>From the archives\r\n" +
	"\r\n" +
	"From bob@example.com Mon Jan  2 15:06:05 2023\r\n" +
	"From: bob@example.com\r\nTo: b@example.com\r\nSubject: third\r\n\r\nbody three\r\n"

func writeMbox(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "takeout.mbox")
	if err := os.WriteFile(p, []byte(fixtureMbox), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMboxSourceSplitsMessages(t *testing.T) {
	src := NewMboxSource(writeMbox(t))
	defer src.Close()
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 messages, got %d", len(items))
	}
	if items[0].Name != "takeout.mbox#00001.eml" {
		t.Errorf("name = %q", items[0].Name)
	}
	for i, want := range []string{"Subject: first", "Subject: second", "Subject: third"} {
		body := readItem(t, items[i])
		if !strings.Contains(body, want) {
			t.Errorf("item %d missing %q:\n%s", i, want, body)
		}
	}
}

func TestMboxSourceImportsAndDedupes(t *testing.T) {
	st := newTestStore(t)
	im := &Importer{Store: st, Paths: newTestPaths(t)}
	ctx := context.Background()

	runOnce := func(id string) *store.Import {
		if err := st.CreateImport(ctx, importStub(id)); err != nil {
			t.Fatal(err)
		}
		src := NewMboxSource(writeMbox(t))
		defer src.Close()
		job := &Job{Importer: im, Hub: NewHub(), Store: st, ID: id}
		job.RunSource(ctx, src)
		imp, err := st.GetImport(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return imp
	}

	first := runOnce("mbox1")
	if first.Processed != 3 || first.Errors != 0 || first.Duplicates != 0 {
		t.Fatalf("first run: %+v", first)
	}
	emails, total, err := st.ListEmails(ctx, store.ListOptions{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(emails) != 3 || total != 3 {
		t.Fatalf("want 3 rows, got %d (total %d)", len(emails), total)
	}

	second := runOnce("mbox2")
	if second.Duplicates != 3 || second.Errors != 0 {
		t.Fatalf("second run should be all duplicates: %+v", second)
	}
}

func TestMboxSourceScanFailsOnMissingFile(t *testing.T) {
	src := NewMboxSource(filepath.Join(t.TempDir(), "nope.mbox"))
	if _, err := src.Scan(context.Background()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}
