package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUsesUserHome(t *testing.T) {
	p, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local-eml")
	if p.Base != want {
		t.Fatalf("Base = %q, want %q", p.Base, want)
	}
	if !strings.HasSuffix(p.EML, "eml") {
		t.Errorf("EML path missing 'eml': %s", p.EML)
	}
	if !strings.HasSuffix(p.DB, "db") {
		t.Errorf("DB path missing 'db': %s", p.DB)
	}
}

func TestEnsureDirsCreatesAll(t *testing.T) {
	tmp := t.TempDir()
	p := Paths{
		Base: tmp,
		EML:  filepath.Join(tmp, "eml"),
		DB:   filepath.Join(tmp, "db"),
		Logs: filepath.Join(tmp, "logs"),
	}
	if err := p.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	for _, d := range []string{p.EML, p.DB, p.Logs} {
		info, err := os.Stat(d)
		if err != nil {
			t.Errorf("missing dir %s: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("not a dir: %s", d)
		}
	}
}

func TestBlobFor(t *testing.T) {
	p := Paths{EML: filepath.Join("tmp", "eml")}
	want := filepath.Join("tmp", "eml", "abc123.eml")
	if got := p.BlobFor("abc123"); got != want {
		t.Errorf("BlobFor = %q, want %q", got, want)
	}
}

func TestDBFile(t *testing.T) {
	p := Paths{DB: filepath.Join("tmp", "db")}
	want := filepath.Join("tmp", "db", "local-eml.db")
	if got := p.DBFile(); got != want {
		t.Errorf("DBFile = %q, want %q", got, want)
	}
}
