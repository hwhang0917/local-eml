package importer

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func readItem(t *testing.T, it Item) string {
	t.Helper()
	rc, err := it.Open(context.Background())
	if err != nil {
		t.Fatalf("open %s: %v", it.Name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", it.Name, err)
	}
	return string(b)
}

func TestLocalFilesSourceFiltersEMLWhenRequested(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "a.eml")
	bad := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(good, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}

	src := NewLocalSource("2 files", []string{good, bad}, []string{"a.eml", "notes.txt"}, true)
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 eml item, got %d", len(items))
	}
	if items[0].Name != "a.eml" {
		t.Errorf("name = %q", items[0].Name)
	}
	if got := readItem(t, items[0]); got != "hello" {
		t.Errorf("body = %q", got)
	}
}

func TestLocalFilesSourceKeepsNonEMLWhenFilterOff(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "single")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := NewLocalSource("upload", []string{p}, []string{"single.eml"}, false)
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1, got %d", len(items))
	}
	if got := readItem(t, items[0]); got != "x" {
		t.Errorf("body = %q, want x", got)
	}
}

func TestZipSourceScansEMLEntries(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "c.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"inbox/one.eml": "ONE",
		"inbox/two.eml": "TWO",
		"readme.txt":    "skip",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	src := NewZipSource(zipPath)
	defer src.Close()
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 eml items, got %d", len(items))
	}
	got := map[string]string{}
	for _, it := range items {
		if filepath.Ext(it.Name) != ".eml" {
			t.Errorf("unexpected entry %q", it.Name)
		}
		got[it.Name] = readItem(t, it)
	}
	if got["one.eml"] != "ONE" {
		t.Errorf("one.eml body = %q, want ONE", got["one.eml"])
	}
	if got["two.eml"] != "TWO" {
		t.Errorf("two.eml body = %q, want TWO", got["two.eml"])
	}
}
