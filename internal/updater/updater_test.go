package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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

func TestDownloadVerifiesChecksum(t *testing.T) {
	payload := []byte("new binary bytes")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bin":
			_, _ = w.Write(payload)
		case "/SHA256SUMS":
			sum := sha256.Sum256(payload)
			fmt.Fprintf(w, "%x  local-eml-linux-amd64\n%x  other\n", sum, sha256.Sum256([]byte("x")))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	sums, err := FetchChecksums(ctx, srv.URL+"/SHA256SUMS")
	if err != nil {
		t.Fatal(err)
	}
	want := sums["local-eml-linux-amd64"]
	if want == "" {
		t.Fatal("checksum for asset not parsed")
	}

	dir := t.TempDir()
	tmp, err := Download(ctx, srv.URL+"/bin", want, dir)
	if err != nil {
		t.Fatalf("matching checksum rejected: %v", err)
	}
	if _, err := os.Stat(tmp); err != nil {
		t.Fatalf("verified file missing: %v", err)
	}

	if _, err := Download(ctx, srv.URL+"/bin", strings.Repeat("0", 64), dir); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("tampered download must be rejected, got err=%v", err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, ".update-*"))
	if len(left) != 1 {
		t.Fatalf("rejected download must be deleted; temp files left: %v", left)
	}
}

func TestManagedByWinget(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		// Portable packages, per-user and machine-wide scope.
		{`C:\Users\me\AppData\Local\Microsoft\WinGet\Packages\hwhang0917.LocalEml_Microsoft.Winget.Source_8wekyb3d8bbwe\local-eml.exe`, true},
		{`C:\Program Files\WinGet\Packages\hwhang0917.LocalEml_Microsoft.Winget.Source_8wekyb3d8bbwe\local-eml.exe`, true},
		// The command alias winget puts on PATH, when not resolved to its target.
		{`C:\Users\me\AppData\Local\Microsoft\WinGet\Links\local-eml.exe`, true},
		{`c:/users/me/appdata/local/microsoft/winget/packages/x/local-eml.exe`, true},
		// Our own installer and a plain download.
		{`C:\Users\me\AppData\Local\local-eml\local-eml.exe`, false},
		{`C:\Users\me\Downloads\local-eml-windows-amd64.exe`, false},
		{`/home/me/.local/bin/local-eml`, false},
	} {
		if got := ManagedByWinget(tc.path); got != tc.want {
			t.Errorf("ManagedByWinget(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
