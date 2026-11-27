// Package tempsession manages the throwaway data folders of temporary
// sessions. Each lives under the OS temp directory with a pid file naming its
// owner, so a later launch can tell a crashed session's leftovers from a
// session that is still running.
//
// A session the user decided to keep (by turning temporary sessions off from
// inside it) carries a keep file too. Its folder is never swept: its mail is
// merged into the real library instead, see Merge.
package tempsession

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	prefix      = "local-eml-session-"
	pidFileName = "session.pid"
	keepName    = "keep"

	// A folder with no pid file may belong to a launch that is between
	// MkdirTemp and writing it; give it this long before calling it orphaned.
	youngGrace = time.Minute

	// Windows refuses to delete a file another handle still has open, and a
	// background job can hold one for a moment after shutdown begins. The
	// console close event allows about five seconds in total, so retry briefly
	// rather than give up on the first sharing violation.
	removeAttempts = 10
	removeBackoff  = 300 * time.Millisecond
)

// Create makes a new session folder under root (normally os.TempDir()) and
// records the current process as its owner.
func Create(root string) (string, error) {
	dir, err := os.MkdirTemp(root, prefix+"*")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, pidFileName), []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		_ = os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}

// Sweep deletes session folders under root whose owner is gone, which is what
// a crash, a kill or a power cut leaves behind. It returns what it removed.
// Failures are skipped: the next launch simply tries again.
func Sweep(root string) []string {
	matches, _ := filepath.Glob(filepath.Join(root, prefix+"*"))
	var removed []string
	for _, dir := range matches {
		if info, ok := sessionDir(dir); !ok || IsKept(dir) || inUse(dir, info) {
			continue
		}
		if os.RemoveAll(dir) == nil {
			removed = append(removed, dir)
		}
	}
	return removed
}

// SetKeep marks dir's data to be kept, or clears the mark. The mark is written
// as soon as the user asks, not at exit, so a crash afterwards keeps the data
// too.
func SetKeep(dir string, keep bool) error {
	p := filepath.Join(dir, keepName)
	if keep {
		return os.WriteFile(p, nil, 0o600)
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsKept reports whether dir is marked to be kept.
func IsKept(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, keepName))
	return err == nil
}

// Kept lists the kept session folders under root whose owner is gone: the
// ones waiting to be merged into the library.
func Kept(root string) []string {
	matches, _ := filepath.Glob(filepath.Join(root, prefix+"*"))
	var kept []string
	for _, dir := range matches {
		info, ok := sessionDir(dir)
		if !ok || !IsKept(dir) || inUse(dir, info) {
			continue
		}
		kept = append(kept, dir)
	}
	return kept
}

// sessionDir reports whether dir is a real session folder of this account's.
// Lstat, not Stat: a symlink with a session's name must not lead the sweep or
// the merge somewhere else.
func sessionDir(dir string) (os.FileInfo, bool) {
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || !ownedByMe(info) {
		return nil, false
	}
	return info, true
}

func inUse(dir string, info os.FileInfo) bool {
	b, err := os.ReadFile(filepath.Join(dir, pidFileName))
	if err != nil {
		return time.Since(info.ModTime()) < youngGrace
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return false
	}
	// ponytail: a reused pid keeps a dead session's folder until that process
	// exits too. Erring toward keeping is the safe direction here.
	return pid == os.Getpid() || processAlive(pid)
}

// Remove deletes a session folder, retrying while files are still held open.
func Remove(dir string) error {
	var err error
	for i := 0; i < removeAttempts; i++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(removeBackoff)
	}
	return err
}
