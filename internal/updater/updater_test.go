package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.2.0", "v0.3.0", true},
		{"0.2.0", "v0.3.0", true},
		{"v0.3.0", "v0.3.0", false},
		{"v0.3.1", "v0.3.0", false},
		{"v0.3.0", "v1.0.0", true},
		{"dev", "v0.3.0", false}, // non-semver current → never upgrade
		{"v0.3.0", "abc", false}, // non-semver latest → don't claim newer
	}
	for _, c := range cases {
		got := IsNewer(c.current, c.latest)
		if got != c.want {
			t.Errorf("IsNewer(%q,%q)=%v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	name := AssetName()
	want := "local-eml-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if name != want {
		t.Errorf("AssetName=%q want %q", name, want)
	}
}

func TestSwapInPlace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rename-while-open semantics differ on Windows")
	}
	dir := t.TempDir()
	current := filepath.Join(dir, "bin")
	if err := writeFile(current, "OLD"); err != nil {
		t.Fatal(err)
	}
	next := filepath.Join(dir, ".update-xyz")
	if err := writeFile(next, "NEW"); err != nil {
		t.Fatal(err)
	}
	if err := Swap(current, next); err != nil {
		t.Fatalf("Swap: %v", err)
	}
	got, err := readFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if got != "NEW" {
		t.Errorf("got %q want NEW", got)
	}
}

func writeFile(p, s string) error {
	return os.WriteFile(p, []byte(s), 0o644)
}

func readFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
