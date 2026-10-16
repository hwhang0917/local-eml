package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

// s3UploadAPI is the subset of *s3.Client used here; lets tests inject a fake.
type s3UploadAPI interface {
	importer.S3Lister
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// S3Job uploads the library to a bucket. It embeds the importer's Job for the
// shared progress stream and status bookkeeping; Job.Importer stays nil.
type S3Job struct {
	importer.Job
	Exporter *Exporter
	Cfg      importer.S3Config

	newClient func(context.Context, importer.S3Config) (s3UploadAPI, error)
}

func (e *Exporter) NewS3Job(id string, cfg importer.S3Config) *S3Job {
	return &S3Job{
		Job:       importer.Job{Hub: e.Hub, Store: e.Store, ID: id},
		Exporter:  e,
		Cfg:       cfg,
		newClient: newS3UploadClient,
	}
}

func newS3UploadClient(ctx context.Context, cfg importer.S3Config) (s3UploadAPI, error) {
	return importer.NewS3Client(ctx, cfg)
}

func (j *S3Job) Run(ctx context.Context) {
	defer j.Hub.Close(j.ID)
	log := j.Log().With(
		slog.String("bucket", j.Cfg.Bucket), slog.String("prefix", j.Cfg.Prefix))
	start := time.Now()
	log.Info("s3 export started")

	client, err := j.newClient(ctx, j.Cfg)
	if err != nil {
		j.fail("aws config", err)
		return
	}

	j.Publish(importer.Event{Type: importer.EventStep, Phase: "Listing existing objects in destination"})
	dedup, err := buildDedup(ctx, client, j.Cfg.Bucket, j.Cfg.Prefix)
	if err != nil {
		j.fail("list destination", err)
		return
	}
	log.Info("destination scan complete",
		slog.Int("full_keys", dedup.fullCount()),
		slog.Int("legacy_short_shas", dedup.legacyCount()))

	j.Publish(importer.Event{Type: importer.EventStep, Phase: "Listing local emails"})
	entries, err := j.Exporter.Store.AllExportEntries(ctx)
	if err != nil {
		j.fail("list emails", err)
		return
	}
	total := len(entries)
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.Publish(importer.Event{Type: importer.EventStep,
		Phase: fmt.Sprintf("Uploading %d emails", total), Total: total})

	var uploaded, duplicates, errs int
	cancelled := false
	for i, em := range entries {
		if err := ctx.Err(); err != nil {
			cancelled = true
			log.Warn("export cancelled", slog.Int("processed", i))
			break
		}
		key := j.Cfg.Prefix + s3ObjectName(em.SHA256)
		idx := i + 1
		if dedup.has(em.SHA256) {
			duplicates++
			j.RecordItem(ctx, key, em.SHA256, true, idx, total)
			continue
		}
		if err := j.uploadOne(ctx, client, em, key); err != nil {
			errs++
			j.RecordItemError(ctx, key, err, idx, total)
			log.Warn("upload failed",
				slog.String("key", key), slog.String("err", err.Error()))
			continue
		}
		uploaded++
		j.RecordItem(ctx, key, em.SHA256, false, idx, total)
	}

	if cancelled {
		j.MarkCancelled(uploaded+duplicates+errs, total)
	} else {
		j.Publish(importer.Event{Type: importer.EventStep, Phase: "Uploading database snapshot"})
		if err := j.uploadDB(ctx, client); err != nil {
			errs++
			_ = j.Store.RecordImportError(ctx, j.ID, j.Cfg.Prefix+DBObjectName, err.Error())
			_ = j.Store.IncImportCounters(ctx, j.ID, 0, 0, 1)
			log.Warn("db snapshot upload failed", slog.String("err", err.Error()))
		}
		j.MarkDone(total)
	}
	log.Info("s3 export finished",
		slog.Int("uploaded", uploaded), slog.Int("duplicates", duplicates),
		slog.Int("errors", errs), slog.Int("total", total),
		slog.Duration("elapsed", time.Since(start)))
}

func (j *S3Job) uploadOne(ctx context.Context, client s3UploadAPI, em store.ExportEntry, key string) error {
	f, err := os.Open(j.Exporter.Paths.BlobFor(em.SHA256))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(j.Cfg.Bucket),
		Key:         aws.String(key),
		Body:        f,
		ContentType: aws.String("message/rfc822"),
	})
	return err
}

// uploadDB puts a fresh snapshot of the metadata database at
// "<prefix>local-eml.db". Unlike the content-addressed emails it changes
// between runs, so it is re-uploaded every export — the dedup indexer only
// looks at .eml keys and never sees it.
func (j *S3Job) uploadDB(ctx context.Context, client s3UploadAPI) error {
	p, cleanup, err := j.Exporter.snapshotDB(ctx)
	if err != nil {
		return err
	}
	defer cleanup()
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(j.Cfg.Bucket),
		Key:         aws.String(j.Cfg.Prefix + DBObjectName),
		Body:        f,
		ContentType: aws.String("application/vnd.sqlite3"),
	})
	return err
}

func (j *S3Job) fail(phase string, cause error) {
	j.Fail(phase, cause)
	j.Log().Error("s3 export failed",
		slog.String("phase", phase), slog.String("err", cause.Error()))
}

// s3Dedup tracks which emails already live in the destination so we can skip
// re-uploading them. Two formats are recognized:
//
//  1. Current format: "<prefix><sha>.eml" — the canonical content-addressed
//     key. Indexed by full 64-char SHA.
//  2. Legacy format: "<prefix><sha[:8]>_<basename>.eml" — what older buggy
//     builds wrote. Indexed by 8-char SHA prefix; we accept a match because
//     a SHA-256 collision in the first 4 bytes of a single user's library is
//     vanishingly unlikely, and the cost of a false positive (one skipped
//     upload) is much smaller than the cost of a false negative (a duplicate
//     piled on top of the user's already-duplicated bucket).
type s3Dedup struct {
	full   map[string]bool // full 64-char SHA
	legacy map[string]bool // 8-char SHA prefix
}

func (d *s3Dedup) has(sha string) bool {
	if d.full[sha] {
		return true
	}
	return len(sha) >= shortSHALen && d.legacy[sha[:shortSHALen]]
}

func (d *s3Dedup) fullCount() int   { return len(d.full) }
func (d *s3Dedup) legacyCount() int { return len(d.legacy) }

func buildDedup(ctx context.Context, client importer.S3Lister, bucket, prefix string) (*s3Dedup, error) {
	d := &s3Dedup{full: map[string]bool{}, legacy: map[string]bool{}}
	err := importer.ListKeys(ctx, client, bucket, prefix, func(key string) {
		indexKey(d, key, prefix)
	})
	if err != nil {
		return nil, err
	}
	return d, nil
}

func indexKey(d *s3Dedup, key, prefix string) {
	rel := strings.TrimPrefix(key, prefix)
	if !importer.IsEML(rel) {
		return
	}
	base := rel[:len(rel)-len(".eml")]
	switch {
	case len(base) == shaHexLen && isHexLower(base):
		d.full[base] = true
	case len(base) > shortSHALen+1 && base[shortSHALen] == '_' && isHexLower(base[:shortSHALen]):
		d.legacy[base[:shortSHALen]] = true
	}
}

func isHexLower(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
