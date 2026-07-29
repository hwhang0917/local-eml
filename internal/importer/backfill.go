package importer

import (
	"context"
	"os"

	"github.com/hwhang0917/local-eml/internal/parser"
)

// attachmentCountVersion marks the one-time re-count of attachment_count after
// inline cid: images stopped counting as attachments. Bump only if stored
// attachment data ever has to be recomputed from the blobs again.
const attachmentCountVersion = 1

// BackfillAttachmentCounts re-parses every stored blob whose recorded
// attachment count predates the inline/attachment split and rewrites the two
// derived columns. Runs once per database, tracked via PRAGMA user_version;
// safe to interrupt — an unfinished run simply repeats on the next start.
func (im *Importer) BackfillAttachmentCounts(ctx context.Context) (int, error) {
	v, err := im.Store.UserVersion(ctx)
	if err != nil || v >= attachmentCountVersion {
		return 0, err
	}
	rows, err := im.Store.ListEmailAttachmentCounts(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return updated, err
		}
		f, err := os.Open(im.Paths.BlobFor(r.SHA256))
		if err != nil {
			continue // missing blob: the drift-repair flow owns that case
		}
		env, err := parser.Open(f)
		f.Close()
		if err != nil {
			continue // unparseable blob keeps its old count
		}
		if n := len(env.Attachments); n != r.AttachmentCount {
			if err := im.Store.SetEmailAttachmentCount(ctx, r.ID, n); err != nil {
				return updated, err
			}
			updated++
		}
	}
	return updated, im.Store.SetUserVersion(ctx, attachmentCountVersion)
}
