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
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("buffer: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, err
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
