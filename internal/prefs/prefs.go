// Package prefs holds the few launch preferences that must outlive the
// library itself. They live in a small JSON file beside the database rather
// than in it, because a temporary session runs on a throwaway database: a
// switch stored there would be deleted along with the mail it governs.
package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Prefs struct {
	// TemporaryLaunches makes every double-click launch run in a throwaway
	// folder that is deleted on exit. The installed service ignores it.
	TemporaryLaunches bool `json:"temporary_launches"`
}

// The file holds no mail or credentials, but it sits in the private data
// folder and says how the user runs the app, so it gets the same mode.
const (
	fileMode = 0o600
	dirMode  = 0o700
)

// Load reads the prefs file. A missing file is the zero value, not an error:
// that's every install that never touched a preference.
func Load(path string) (Prefs, error) {
	var p Prefs
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(b, &p)
}

// Save writes p atomically: a crash mid-write leaves the old file, never a
// truncated one that would fail to parse on the next launch.
func Save(path string, p Prefs) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, fileMode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
