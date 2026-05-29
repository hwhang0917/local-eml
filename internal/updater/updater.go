// Package updater checks GitHub for newer Local Eml releases and replaces the
// running binary in place. The replacement is verified against the release's
// SHA256SUMS file before being swapped over the live executable. On Linux and
// macOS the swap uses os.Rename, which is atomic even while the binary is
// mapped — the kernel keeps the old inode alive for the running process and
// the next launch picks up the new file. On Windows the live binary can't be
// overwritten while open, so we rename the old one aside first.
//
// The package never restarts the process itself; the caller decides when to
// exit (typically after telling the service manager to bring us back up).
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	releasesAPI    = "https://api.github.com/repos/hwhang0917/local-eml/releases/latest"
	checksumsName  = "SHA256SUMS"
	httpUserAgent  = "local-eml-updater"
	httpListMaxAge = 30 * time.Second
	httpDLTimeout  = 5 * time.Minute
)

type Release struct {
	Tag    string  `json:"tag_name"`
	URL    string  `json:"html_url"`
	Body   string  `json:"body"`
	Assets []Asset `json:"assets"`
}

type Asset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"browser_download_url"`
	Size        int64  `json:"size"`
}

// LatestRelease returns the latest published Local Eml release on GitHub.
func LatestRelease(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", httpUserAgent)

	client := &http.Client{Timeout: httpListMaxAge}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases api: %s", res.Status)
	}
	var rel Release
	if err := json.NewDecoder(res.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}

// IsNewer reports whether latestTag is strictly greater than currentTag using
// X.Y.Z semver order. Either input may carry a leading "v". Non-semver inputs
// (e.g. "dev" during a development build) return false so the caller treats
// "I'm not sure" the same as "no update available."
func IsNewer(currentTag, latestTag string) bool {
	cv, ok1 := parseSemver(currentTag)
	lv, ok2 := parseSemver(latestTag)
	if !ok1 || !ok2 {
		return false
	}
	for i := range cv {
		if lv[i] > cv[i] {
			return true
		}
		if lv[i] < cv[i] {
			return false
		}
	}
	return false
}

func parseSemver(tag string) ([3]int, bool) {
	s := strings.TrimPrefix(strings.TrimSpace(tag), "v")
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var v [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		v[i] = n
	}
	return v, true
}

// AssetName returns the expected binary asset name for the running OS/arch.
func AssetName() string {
	name := fmt.Sprintf("local-eml-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// FindAsset returns the asset matching the running platform plus the SHA256SUMS
// asset, or an error explaining which one is missing.
func (r *Release) FindAsset() (binary, checksums *Asset, err error) {
	want := AssetName()
	for i, a := range r.Assets {
		switch a.Name {
		case want:
			binary = &r.Assets[i]
		case checksumsName:
			checksums = &r.Assets[i]
		}
	}
	if binary == nil {
		return nil, nil, fmt.Errorf("no asset %q in release %s", want, r.Tag)
	}
	if checksums == nil {
		return nil, nil, fmt.Errorf("no %s in release %s", checksumsName, r.Tag)
	}
	return binary, checksums, nil
}

// FetchChecksums downloads and parses a SHA256SUMS file ("sha  name" per line).
func FetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	body, err := httpGet(ctx, url, 30*time.Second)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		out[fields[1]] = strings.ToLower(fields[0])
	}
	return out, nil
}

// Download fetches url into a temp file beside targetDir, verifies its SHA256
// against expected (lowercase hex), and returns the path of the verified temp
// file. The caller owns deletion on failure (Swap handles it on success).
func Download(ctx context.Context, url, expectedSHA, targetDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", httpUserAgent)
	client := &http.Client{Timeout: httpDLTimeout}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", url, res.Status)
	}
	f, err := os.CreateTemp(targetDir, ".update-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), res.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expectedSHA) {
		os.Remove(tmp)
		return "", fmt.Errorf("checksum mismatch: got %s, want %s", got, expectedSHA)
	}
	return tmp, nil
}

// Swap replaces currentPath with newPath using the platform's best strategy.
// On success the old binary is removed (Linux/macOS via in-place rename) or
// renamed aside as currentPath+".old" (Windows). The new file ends up with
// mode 0755 so the OS can execute it after restart.
func Swap(currentPath, newPath string) error {
	if err := os.Chmod(newPath, 0o755); err != nil {
		return fmt.Errorf("chmod new: %w", err)
	}
	if runtime.GOOS == "windows" {
		backup := currentPath + ".old"
		_ = os.Remove(backup) // residue from a prior update
		if err := os.Rename(currentPath, backup); err != nil {
			return fmt.Errorf("rename current to backup: %w", err)
		}
		if err := os.Rename(newPath, currentPath); err != nil {
			// Try to undo so we don't leave the user with no binary at all.
			_ = os.Rename(backup, currentPath)
			return fmt.Errorf("rename new to current: %w", err)
		}
		return nil
	}
	if err := os.Rename(newPath, currentPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// CurrentBinary returns the absolute path of the running binary. Resolves
// symlinks so the swap targets the real on-disk file, not a launcher link.
func CurrentBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe, nil
	}
	return resolved, nil
}

func httpGet(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", httpUserAgent)
	client := &http.Client{Timeout: timeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, res.Status)
	}
	return io.ReadAll(res.Body)
}

// ErrNotInService is returned by Install handlers when the caller is running
// outside a managed service, so a graceful self-restart isn't available.
var ErrNotInService = errors.New("not running as a managed service; cannot self-restart")
