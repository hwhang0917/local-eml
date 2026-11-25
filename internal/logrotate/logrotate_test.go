package logrotate

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func day(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func names(t *testing.T, dir string) []string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func open(t *testing.T, dir string, c *clock) *Writer {
	t.Helper()
	w, err := Open(dir, "app.log", Options{Now: c.now, OnError: func(err error) { t.Errorf("onError: %v", err) }})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestRotatesAtMidnightWithoutLosingLines(t *testing.T) {
	dir := t.TempDir()
	c := &clock{day("2026-11-24 23:59")}
	w := open(t, dir, c)
	w.Write([]byte("a\n"))
	c.t = day("2026-11-25 00:00")
	w.Write([]byte("b\n"))
	w.Close()

	if got := read(t, filepath.Join(dir, "app-2026-11-24.log")); got != "a\n" {
		t.Errorf("rotated = %q", got)
	}
	if got := read(t, filepath.Join(dir, "app.log")); got != "b\n" {
		t.Errorf("active = %q", got)
	}
}

func TestRestartSameDayAppends(t *testing.T) {
	dir := t.TempDir()
	c := &clock{time.Now()}
	w := open(t, dir, c)
	w.Write([]byte("a\n"))
	w.Close()
	w = open(t, dir, c)
	w.Write([]byte("b\n"))
	w.Close()

	if got := names(t, dir); len(got) != 1 {
		t.Fatalf("files = %v", got)
	}
	if got := read(t, filepath.Join(dir, "app.log")); got != "a\nb\n" {
		t.Errorf("active = %q", got)
	}
}

func TestRestartOnNewDayRotatesOldFile(t *testing.T) {
	dir := t.TempDir()
	active := filepath.Join(dir, "app.log")
	if err := os.WriteFile(active, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mtime := day("2026-11-20 10:00")
	os.Chtimes(active, mtime, mtime)

	w := open(t, dir, &clock{day("2026-11-25 09:00")})
	w.Write([]byte("new\n"))
	w.Close()

	if got := read(t, filepath.Join(dir, "app-2026-11-20.log")); got != "old\n" {
		t.Errorf("rotated = %q", got)
	}
	if got := read(t, active); got != "new\n" {
		t.Errorf("active = %q", got)
	}
}

func TestSameDayRotationDoesNotOverwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app-2026-11-24.log"), []byte("first\n"), 0o600)
	c := &clock{day("2026-11-24 22:00")}
	w := open(t, dir, c)
	w.Write([]byte("second\n"))
	c.t = day("2026-11-25 01:00")
	w.Write([]byte("x\n"))
	w.Close()

	if got := read(t, filepath.Join(dir, "app-2026-11-24.log")); got != "first\n" {
		t.Errorf("original = %q", got)
	}
	if got := read(t, filepath.Join(dir, "app-2026-11-24.1.log")); got != "second\n" {
		t.Errorf("suffixed = %q", got)
	}
}

func TestPrunesOlderThanRetention(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{
		"app-2026-10-25.log",   // 31 days old: delete
		"app-2026-10-25.2.log", // same, suffixed: delete
		"app-2026-10-26.log",   // 30 days old: keep
		"app-2026-11-24.log",   // keep
		"app-notes.log",        // not ours: keep
		"other-2020-01-01.log", // not ours: keep
	} {
		os.WriteFile(filepath.Join(dir, n), nil, 0o600)
	}
	c := &clock{day("2026-11-25 08:00")}
	w := open(t, dir, c)
	w.Close()

	want := []string{"app-2026-10-26.log", "app-2026-11-24.log", "app-notes.log", "app.log", "other-2020-01-01.log"}
	if got := names(t, dir); !equal(got, want) {
		t.Errorf("files = %v, want %v", got, want)
	}
}

func TestPrunesOnRotation(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app-2026-10-25.log"), nil, 0o600)
	c := &clock{day("2026-11-24 12:00")} // 2026-10-25 is exactly 30 days old: kept
	w := open(t, dir, c)
	if _, err := os.Stat(filepath.Join(dir, "app-2026-10-25.log")); err != nil {
		t.Fatalf("pruned too early: %v", err)
	}
	c.t = day("2026-11-25 00:01")
	w.Write([]byte("x\n"))
	w.Close()
	if _, err := os.Stat(filepath.Join(dir, "app-2026-10-25.log")); !os.IsNotExist(err) {
		t.Errorf("not pruned on rotation: %v", err)
	}
}

func TestWriteAfterClose(t *testing.T) {
	w := open(t, t.TempDir(), &clock{time.Now()})
	w.Close()
	if _, err := w.Write([]byte("x")); err == nil {
		t.Error("write after close succeeded")
	}
}

func TestRotatedDate(t *testing.T) {
	for name, want := range map[string]string{
		"app-2026-11-24.log":    "2026-11-24",
		"app-2026-11-24.12.log": "2026-11-24",
		"app-2026-11-24..log":   "",
		"app-2026-11-24.x.log":  "",
		"app-2026-13-01.log":    "",
		"app-2026-11-24.txt":    "",
		"app.log":               "",
	} {
		got, ok := RotatedDate(name, "app-", ".log")
		if got != want || ok != (want != "") {
			t.Errorf("%s: got %q %v", name, got, ok)
		}
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
