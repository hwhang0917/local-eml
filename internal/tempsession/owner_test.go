//go:build !windows

package tempsession

import (
	"os"
	"path/filepath"
	"testing"
)

// The temp directory is shared: only folders this account made, closed to
// everyone else, may be merged into the library or swept.
func TestKeptIgnoresFoldersThatAreNotOurs(t *testing.T) {
	root := t.TempDir()
	mk := func(name string, mode os.FileMode) string {
		dir := filepath.Join(root, prefix+name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := SetKeep(dir, true); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, pidFileName), []byte("2147483646"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, mode); err != nil {
			t.Fatal(err)
		}
		return dir
	}
	mine := mk("mine", 0o700)
	open := mk("open", 0o755)
	link := filepath.Join(root, prefix+"link")
	if err := os.Symlink(mine, link); err != nil {
		t.Fatal(err)
	}

	kept := Kept(root)
	if len(kept) != 1 || kept[0] != mine {
		t.Fatalf("Kept = %v, want only %s (not %s or %s)", kept, mine, open, link)
	}
}
