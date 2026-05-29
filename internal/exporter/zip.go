package exporter

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

// WriteZip streams every email in the library as a single zip into w.
// Errors per file are skipped but logged; the overall operation only fails on
// a writer-level error (which usually means the client disconnected).
func (e *Exporter) WriteZip(ctx context.Context, w io.Writer) (written, skipped int, err error) {
	entries, err := e.Store.AllExportEntries(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list emails: %w", err)
	}

	zw := zip.NewWriter(w)
	defer func() {
		if cerr := zw.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	log := slog.Default().With(slog.Int("count", len(entries)))
	log.Info("zip export started")
	start := time.Now()

	for _, em := range entries {
		if err := ctx.Err(); err != nil {
			return written, skipped, err
		}
		if werr := writeOne(zw, e.Paths.BlobFor(em.SHA256), zipObjectName(em.SHA256, em.Filename)); werr != nil {
			log.Warn("zip entry failed",
				slog.String("sha256", em.SHA256), slog.String("err", werr.Error()))
			skipped++
			continue
		}
		written++
	}

	log.Info("zip export finished",
		slog.Int("written", written), slog.Int("skipped", skipped),
		slog.Duration("elapsed", time.Since(start)))
	return written, skipped, nil
}

func writeOne(zw *zip.Writer, srcPath, name string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	stat, err := src.Stat()
	if err != nil {
		return err
	}
	header := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: stat.ModTime(),
	}
	dst, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return err
}
