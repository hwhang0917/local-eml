package importer

import (
	"context"
	"os"

	"github.com/hwhang0917/local-eml/internal/parser"
)

// One-time data repairs that need the blobs re-parsed, each gated on PRAGMA
// user_version so it runs once per database. All are safe to interrupt: an
// unfinished run simply repeats on the next start.
const (
	// attachmentCountVersion marks the re-count of attachment_count after
	// inline cid: images stopped counting as attachments.
	attachmentCountVersion = 1
	// threadIDVersion marks the derivation of thread_id for rows imported
	// before conversations existed.
	threadIDVersion = 2
	// riskVersion marks the phishing assessment of rows imported before the
	// heuristics existed.
	riskVersion = 3
)

// BackfillAttachmentCounts re-parses every stored blob whose recorded
// attachment count predates the inline/attachment split and rewrites the two
// derived columns.
func (im *Importer) BackfillAttachmentCounts(ctx context.Context) (int, error) {
	if due, err := im.due(ctx, attachmentCountVersion); !due {
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
		parsed, ok := im.parseBlob(r.SHA256)
		if !ok || parsed.AttachmentCount == r.AttachmentCount {
			continue
		}
		if err := im.Store.SetEmailAttachmentCount(ctx, r.ID, parsed.AttachmentCount); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, im.Store.SetUserVersion(ctx, attachmentCountVersion)
}

// BackfillThreadIDs re-parses blobs for rows that have no thread_id yet and
// writes the derived key.
func (im *Importer) BackfillThreadIDs(ctx context.Context) (int, error) {
	if due, err := im.due(ctx, threadIDVersion); !due {
		return 0, err
	}
	rows, err := im.Store.ListEmailsMissingThreadID(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return updated, err
		}
		parsed, ok := im.parseBlob(r.SHA256)
		if !ok || parsed.ThreadID == "" {
			continue // nothing to derive a thread from
		}
		if err := im.Store.SetEmailThreadID(ctx, r.ID, parsed.ThreadID); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, im.Store.SetUserVersion(ctx, threadIDVersion)
}

// BackfillRisk runs the phishing heuristics over every row that has never
// been assessed (risk IS NULL).
func (im *Importer) BackfillRisk(ctx context.Context) (int, error) {
	if due, err := im.due(ctx, riskVersion); !due {
		return 0, err
	}
	rows, err := im.Store.ListEmailsMissingRisk(ctx)
	if err != nil {
		return 0, err
	}
	updated := 0
	for _, r := range rows {
		if err := ctx.Err(); err != nil {
			return updated, err
		}
		parsed, ok := im.parseBlob(r.SHA256)
		if !ok {
			continue
		}
		if err := im.Store.SetEmailRisk(ctx, r.ID, parsed.Risk); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, im.Store.SetUserVersion(ctx, riskVersion)
}

// due reports whether the repair tagged version has not run on this database.
func (im *Importer) due(ctx context.Context, version int) (bool, error) {
	v, err := im.Store.UserVersion(ctx)
	if err != nil {
		return false, err
	}
	return v < version, nil
}

// parseBlob re-reads a stored message. A missing blob is the drift-repair
// flow's case and an unparseable one keeps whatever it has, so both report
// false rather than an error.
func (im *Importer) parseBlob(sha string) (*parser.Parsed, bool) {
	f, err := os.Open(im.Paths.BlobFor(sha))
	if err != nil {
		return nil, false
	}
	defer f.Close()
	parsed, err := parser.Parse(f)
	if err != nil {
		return nil, false
	}
	return parsed, true
}
