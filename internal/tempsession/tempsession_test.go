package tempsession

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestCreateWritesOwnPID(t *testing.T) {
	root := t.TempDir()
	dir, err := Create(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, pidFileName))
	if err != nil || string(b) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("pid file = %q, %v", b, err)
	}
}

func TestSweepRemovesOnlyDeadSessions(t *testing.T) {
	root := t.TempDir()
	live, err := Create(root)
	if err != nil {
		t.Fatal(err)
	}

	dead := filepath.Join(root, prefix+"dead")
	if err := os.MkdirAll(filepath.Join(dead, "db"), 0o700); err != nil {
		t.Fatal(err)
	}
	// PIDs are far below this on every platform we ship for.
	if err := os.WriteFile(filepath.Join(dead, pidFileName), []byte("2147483646"), 0o600); err != nil {
		t.Fatal(err)
	}

	// No pid file yet but freshly made: another launch may be mid-Create.
	young := filepath.Join(root, prefix+"young")
	if err := os.Mkdir(young, 0o700); err != nil {
		t.Fatal(err)
	}

	// No pid file and old: a launch that died before writing it.
	orphan := filepath.Join(root, prefix+"orphan")
	if err := os.Mkdir(orphan, 0o700); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * youngGrace)
	if err := os.Chtimes(orphan, old, old); err != nil {
		t.Fatal(err)
	}

	unrelated := filepath.Join(root, "someone-elses-folder")
	if err := os.Mkdir(unrelated, 0o700); err != nil {
		t.Fatal(err)
	}

	removed := Sweep(root)
	if len(removed) != 2 {
		t.Fatalf("removed = %v, want the dead and orphan sessions", removed)
	}
	for dir, want := range map[string]bool{live: true, dead: false, young: true, orphan: false, unrelated: true} {
		if _, err := os.Stat(dir); (err == nil) != want {
			t.Errorf("%s: exists = %v, want %v", filepath.Base(dir), err == nil, want)
		}
	}
}

func TestRemoveDeletesTree(t *testing.T) {
	dir, err := Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mail.eml"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Remove(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("session folder still exists: %v", err)
	}
}
