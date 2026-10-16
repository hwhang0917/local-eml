package importer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/hwhang0917/local-eml/internal/parser"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

type Importer struct {
	Store *store.Store
	Paths paths.Paths
}

// MaxMessageBytes caps a single message. Every part of a message is held in
// memory while it is parsed, and an archive entry or a hostile server can
// claim to be any size, so the cap is what keeps one item from taking the
// process down. Mail servers reject messages well below this.
const MaxMessageBytes = 64 << 20

// ErrMessageTooLarge is reported for an item over MaxMessageBytes.
var ErrMessageTooLarge = fmt.Errorf("message exceeds %d MiB", MaxMessageBytes>>20)

type Result struct {
	SHA256    string `json:"sha256"`
	Duplicate bool   `json:"duplicate"`
	EmailID   int64  `json:"email_id,omitempty"`
}

func (im *Importer) ImportReader(ctx context.Context, r io.Reader, originalName string) (*Result, error) {
	tmp, err := os.CreateTemp("", "import-*.eml")
	if err != nil {
		return nil, fmt.Errorf("tempfile: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	// One byte past the cap tells an oversized item apart from an exact fit.
	n, err := io.Copy(tmp, io.LimitReader(r, MaxMessageBytes+1))
	if err != nil {
		tmp.Close()
		return nil, fmt.Errorf("buffer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if n > MaxMessageBytes {
		return nil, ErrMessageTooLarge
	}
	return im.ImportFile(ctx, tmpName, originalName)
}

func (im *Importer) ImportFile(ctx context.Context, srcPath, originalName string) (*Result, error) {
	f, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("hash: %w", err)
	}
	sum := hex.EncodeToString(hasher.Sum(nil))

	exists, err := im.Store.EmailExists(ctx, sum)
	if err != nil {
		return nil, fmt.Errorf("dup check: %w", err)
	}
	if exists {
		return &Result{SHA256: sum, Duplicate: true}, nil
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	blobPath := im.Paths.BlobFor(sum)
	if err := atomicCopy(srcPath, blobPath); err != nil {
		return nil, fmt.Errorf("blob write: %w", err)
	}

	row := store.EmailRow{
		Email: store.Email{
			SHA256:          sum,
			Filename:        originalName,
			Subject:         parsed.Subject,
			FromAddr:        parsed.From,
			ToAddrs:         parsed.To,
			CcAddrs:         parsed.Cc,
			MessageID:       parsed.MessageID,
			ThreadID:        parsed.ThreadID,
			SentAt:          parsed.Date,
			ReceivedAt:      time.Now(),
			SizeBytes:       info.Size(),
			HasAttachments:  parsed.AttachmentCount > 0,
			AttachmentCount: parsed.AttachmentCount,
			Risk:            parsed.Risk,
		},
		BodyText: parsed.BodyText,
	}
	id, err := im.Store.InsertEmail(ctx, row)
	if err != nil {
		if errors.Is(err, store.ErrDuplicate) {
			return &Result{SHA256: sum, Duplicate: true}, nil
		}
		_ = os.Remove(blobPath)
		return nil, fmt.Errorf("insert: %w", err)
	}
	return &Result{SHA256: sum, EmailID: id}, nil
}

func atomicCopy(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*.eml")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, dst)
}
