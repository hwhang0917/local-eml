package exporter

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/hwhang0917/local-eml/internal/store"
)

func TestWriteZip_IncludesDBSnapshotAndEmails(t *testing.T) {
	rows := []store.EmailRow{entry("cafef00d", "inbox/hello.eml")}
	exp, _, _ := newExporterFixture(t, rows)

	var buf bytes.Buffer
	written, skipped, err := exp.WriteZip(context.Background(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	if written != 1 || skipped != 0 {
		t.Fatalf("written=%d skipped=%d, want 1/0", written, skipped)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]*zip.File{}
	for _, f := range zr.File {
		names[f.Name] = f
	}

	db, ok := names[dbObjectName]
	if !ok {
		t.Fatalf("zip entries %v, want %q included", keysOf(names), dbObjectName)
	}
	rc, err := db.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	head := make([]byte, 16)
	if _, err := io.ReadFull(rc, head); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(head), "SQLite format 3") {
		t.Errorf("db entry starts with %q, want SQLite header", head)
	}

	wantEML := rows[0].SHA256[:8] + "_hello.eml"
	if _, ok := names[wantEML]; !ok {
		t.Errorf("zip entries %v, want %q included", keysOf(names), wantEML)
	}
}

func keysOf(m map[string]*zip.File) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
