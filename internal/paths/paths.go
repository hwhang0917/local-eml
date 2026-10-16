package paths

import (
	"os"
	"path/filepath"
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
	return At(filepath.Join(home, baseDirName)), nil
}

// At lays out the data folders under base. Resolve uses the home directory;
// a temporary session points it at a throwaway folder instead.
func At(base string) Paths {
	return Paths{
		Base: base,
		EML:  filepath.Join(base, "eml"),
		DB:   filepath.Join(base, "db"),
		Logs: filepath.Join(base, "logs"),
		Keys: filepath.Join(base, "keys"),
	}
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

// PrefsFile holds launch preferences that must survive a library wipe; see
// package prefs.
func (p Paths) PrefsFile() string {
	return filepath.Join(p.Base, "prefs.json")
}

// TokenFile holds the session token the API requires; see server/auth.go.
// Like the key, it lives in the owner-only folder and survives a wipe.
func (p Paths) TokenFile() string {
	return filepath.Join(p.Base, "session-token")
}

func (p Paths) KeyFile() string {
	return filepath.Join(p.Keys, "secret.key")
}

func (p Paths) BlobFor(sha string) string {
	return filepath.Join(p.EML, sha+".eml")
}
