package tempsession

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

// MergeSummary reports what a merge carried into the library.
type MergeSummary struct {
	Added      int // emails new to the library
	Duplicates int // emails the library already had
	Failed     int // emails that could not be imported; the folder is kept
}

// Merge imports a kept session's mail into the library im writes to, then its
// stars, read marks, categories and profiles (see
// store.MergeSessionMetadata). It is safe to run again on the same folder:
// mail already merged comes back as duplicates.
//
// It reports an error when anything was left behind, so the caller keeps the
// folder and tries again on the next launch rather than delete mail that only
// exists there.
func Merge(ctx context.Context, im *importer.Importer, dir string) (MergeSummary, error) {
	var sum MergeSummary
	src := paths.At(dir)
	if _, err := os.Stat(src.DBFile()); errors.Is(err, os.ErrNotExist) {
		// The session ended before its database was created: nothing to keep.
		return sum, nil
	}
	// Opening, not just attaching, so the session's write-ahead log is
	// checkpointed into the file before anything reads it.
	sst, err := store.Open(ctx, src.DBFile())
	if err != nil {
		return sum, fmt.Errorf("open session library: %w", err)
	}
	entries, err := sst.AllExportEntries(ctx)
	sst.Close()
	if err != nil {
		return sum, fmt.Errorf("list session mail: %w", err)
	}

	var added []string
	var errs []error
	for _, e := range entries {
		res, err := im.ImportFile(ctx, src.BlobFor(e.SHA256), e.Filename)
		if err != nil {
			sum.Failed++
			errs = append(errs, fmt.Errorf("%s: %w", e.SHA256, err))
			if ctx.Err() != nil {
				return sum, ctx.Err()
			}
			continue
		}
		if res.Duplicate {
			sum.Duplicates++
			continue
		}
		sum.Added++
		added = append(added, res.SHA256)
	}
	if _, err := im.Store.MergeSessionMetadata(ctx, src.DBFile(), added); err != nil {
		errs = append(errs, fmt.Errorf("merge metadata: %w", err))
	}
	return sum, errors.Join(errs...)
}
