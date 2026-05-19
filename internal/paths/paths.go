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
	}, nil
}

func (p Paths) EnsureDirs() error {
	for _, d := range []string{p.EML, p.DB, p.Logs} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (p Paths) DBFile() string {
	return filepath.Join(p.DB, "local-eml.db")
}

func (p Paths) BlobFor(sha string) string {
	return filepath.Join(p.EML, sha+".eml")
}
