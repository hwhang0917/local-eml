package prefs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadMissingFileIsZero(t *testing.T) {
	p, err := Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil || p.TemporaryLaunches {
		t.Fatalf("missing file must load as zero prefs: %+v, %v", p, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	// The parent does not exist yet: a temporary session may write the prefs
	// before the persistent data folder was ever created.
	path := filepath.Join(t.TempDir(), "base", "prefs.json")
	if err := Save(path, Prefs{TemporaryLaunches: true}); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil || !p.TemporaryLaunches {
		t.Fatalf("round trip lost the switch: %+v, %v", p, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatalf("prefs file mode = %v, want 0600 (err %v)", info.Mode().Perm(), err)
		}
	}
}

func TestLoadCorruptFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("corrupt prefs must be reported, not silently read as defaults")
	}
}
