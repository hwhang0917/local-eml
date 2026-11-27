package exporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestBackupToFolderIsIncremental(t *testing.T) {
	rows := []store.EmailRow{entry("aa", "a.eml"), entry("bb", "b.eml")}
	exp, _, _ := newExporterFixture(t, rows)
	dir := filepath.Join(t.TempDir(), "My Backup")
	ctx := context.Background()

	res, err := exp.BackupToFolder(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 2 || res.Total != 2 || res.Failed != 0 {
		t.Fatalf("first run %+v", res)
	}
	for _, r := range rows {
		if _, err := os.Stat(filepath.Join(dir, BackupEmailsDir, r.SHA256+".eml")); err != nil {
			t.Errorf("missing %s: %v", r.SHA256, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, DBObjectName)); err != nil {
		t.Errorf("no db snapshot: %v", err)
	}

	// Second run copies nothing; a message deleted from the library stays.
	if err := exp.Store.DeleteEmailBySHA(ctx, rows[0].SHA256); err != nil {
		t.Fatal(err)
	}
	res, err = exp.BackupToFolder(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Added != 0 || res.Total != 1 {
		t.Fatalf("second run %+v, want nothing added", res)
	}
	if _, err := os.Stat(filepath.Join(dir, BackupEmailsDir, rows[0].SHA256+".eml")); err != nil {
		t.Errorf("deleted mail was removed from the backup: %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) > 0 {
		t.Errorf("temp files left: %v", left)
	}
}

func TestCheckBackupDirRejectsRelative(t *testing.T) {
	if err := CheckBackupDir("backups/mail"); err == nil {
		t.Fatal("relative path accepted")
	}
}
