package paths

import (
	"os"
	"path/filepath"
	"strings"
)

const baseDirName = ".local-eml"

type Paths struct {
	Base string
	EML  string
	DB   string
	Logs string
	Keys string
}

func Resolve() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	base := filepath.Join(home, baseDirName)
	return Paths{
		Base: base,
		EML:  filepath.Join(base, "eml"),
		DB:   filepath.Join(base, "db"),
		Logs: filepath.Join(base, "logs"),
		Keys: filepath.Join(base, "keys"),
	}, nil
}

// Everything under the base directory is private mail: the .eml blobs, the
// SQLite database, the logs (which carry subjects and addresses) and the
// encryption key. 0700 on every directory keeps other local users out.
const dirMode = 0o700

func (p Paths) EnsureDirs() error {
	for _, d := range []string{p.Base, p.EML, p.DB, p.Logs, p.Keys} {
		if err := os.MkdirAll(d, dirMode); err != nil {
			return err
		}
		// MkdirAll leaves an existing directory's mode alone, so installs
		// created before this default tightened would keep their 0755 bits
		// forever without an explicit chmod.
		if err := os.Chmod(d, dirMode); err != nil {
			return err
		}
	}
	return nil
}

func (p Paths) DBFile() string {
	return filepath.Join(p.DB, "local-eml.db")
}

func (p Paths) KeyFile() string {
	return filepath.Join(p.Keys, "secret.key")
}

func (p Paths) BlobFor(sha string) string {
	candidate := filepath.Join(p.EML, sha+".eml")

	baseAbs, err := filepath.Abs(p.EML)
	if err != nil {
		return filepath.Join(p.EML, ".invalid.eml")
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return filepath.Join(p.EML, ".invalid.eml")
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Join(p.EML, ".invalid.eml")
	}

	return candidate
}
