package importer

import (
	"context"
	"os"

	"github.com/hwhang0917/local-eml/internal/parser"
	"github.com/hwhang0917/local-eml/internal/risk"
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

// threadIDVersion marks the one-time derivation of thread_id for rows imported
// before conversations existed.
const threadIDVersion = 2

// BackfillThreadIDs re-parses blobs for rows that have no thread_id yet and
// writes the derived key. Same contract as BackfillAttachmentCounts: runs once
// per database via PRAGMA user_version, safe to interrupt.
func (im *Importer) BackfillThreadIDs(ctx context.Context) (int, error) {
	v, err := im.Store.UserVersion(ctx)
	if err != nil || v >= threadIDVersion {
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
		f, err := os.Open(im.Paths.BlobFor(r.SHA256))
		if err != nil {
			continue // missing blob: the drift-repair flow owns that case
		}
		parsed, err := parser.Parse(f)
		f.Close()
		if err != nil || parsed.ThreadID == "" {
			continue // unparseable, or nothing to derive a thread from
		}
		if err := im.Store.SetEmailThreadID(ctx, r.ID, parsed.ThreadID); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, im.Store.SetUserVersion(ctx, threadIDVersion)
}

// riskVersion marks the one-time phishing assessment of rows imported before
// the heuristics existed.
const riskVersion = 3

// BackfillRisk runs the phishing heuristics over every row that has never
// been assessed (risk IS NULL). Same contract as the other backfills: once
// per database via PRAGMA user_version, safe to interrupt.
func (im *Importer) BackfillRisk(ctx context.Context) (int, error) {
	v, err := im.Store.UserVersion(ctx)
	if err != nil || v >= riskVersion {
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
		f, err := os.Open(im.Paths.BlobFor(r.SHA256))
		if err != nil {
			continue // missing blob: the drift-repair flow owns that case
		}
		env, err := parser.Open(f)
		f.Close()
		if err != nil {
			continue // unparseable blob stays unassessed
		}
		if err := im.Store.SetEmailRisk(ctx, r.ID, risk.Assess(env)); err != nil {
			return updated, err
		}
		updated++
	}
	return updated, im.Store.SetUserVersion(ctx, riskVersion)
}
