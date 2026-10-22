package importer

import (
	"archive/zip"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestImportReaderRejectsOversizedMessage(t *testing.T) {
	st := newTestStore(t)
	im := &Importer{Store: st, Paths: newTestPaths(t)}
	// A reader that never ends: without the cap this would fill the disk.
	endless := strings.NewReader(minimalEML)
	_, err := im.ImportReader(context.Background(), &repeatReader{r: endless, src: minimalEML}, "huge.eml")
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("err = %v, want ErrMessageTooLarge", err)
	}
	if entries, _ := os.ReadDir(im.Paths.EML); len(entries) != 0 {
		t.Fatalf("oversized message must not be stored: %d blobs", len(entries))
	}
}

// repeatReader serves src forever.
type repeatReader struct {
	r   *strings.Reader
	src string
}

func (rr *repeatReader) Read(p []byte) (int, error) {
	n, err := rr.r.Read(p)
	if err != nil {
		rr.r.Reset(rr.src)
		return rr.r.Read(p)
	}
	return n, nil
}

func TestZipEntryDeclaredOversizedIsRefused(t *testing.T) {
	// The header claims the size: a bomb claims small and inflates big, an
	// honest oversized entry claims big. The claim is checked before inflating
	// and the spool's own cap catches the rest.
	it := zipItem(&zip.File{FileHeader: zip.FileHeader{Name: "dir/big.eml", UncompressedSize64: MaxMessageBytes + 1}})
	if it.Name != "big.eml" {
		t.Fatalf("name = %q", it.Name)
	}
	if _, err := it.Open(context.Background()); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("open = %v, want ErrMessageTooLarge", err)
	}
}
