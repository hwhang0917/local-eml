package exporter

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/store"
)

type S3Config struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Bucket          string
	Prefix          string
}

// s3UploadAPI is the subset of *s3.Client used here; lets tests inject a fake.
type s3UploadAPI interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	PutObject(context.Context, *s3.PutObjectInput, ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type S3Job struct {
	Exporter *Exporter
	ID       string
	Cfg      S3Config
	Logger   *slog.Logger

	newClient func(context.Context, S3Config) (s3UploadAPI, error)
}

func (e *Exporter) NewS3Job(id string, cfg S3Config) *S3Job {
	return &S3Job{Exporter: e, ID: id, Cfg: cfg, newClient: defaultS3UploadClient}
}

func (j *S3Job) logger() *slog.Logger {
	if j.Logger != nil {
		return j.Logger
	}
	return slog.Default()
}

func (j *S3Job) Run(ctx context.Context) {
	defer j.Exporter.Hub.Close(j.ID)
	log := j.logger().With(
		slog.String("bucket", j.Cfg.Bucket), slog.String("prefix", j.Cfg.Prefix))
	start := time.Now()
	log.Info("s3 export started")

	client, err := j.newClient(ctx, j.Cfg)
	if err != nil {
		j.fail(ctx, "aws config", err)
		return
	}

	j.publish(importer.Event{Type: "step", Phase: "Listing existing objects in destination"})
	dedup, err := buildDedup(ctx, client, j.Cfg.Bucket, j.Cfg.Prefix)
	if err != nil {
		j.fail(ctx, "list destination", err)
		return
	}
	log.Info("destination scan complete",
		slog.Int("full_keys", dedup.fullCount()),
		slog.Int("legacy_short_shas", dedup.legacyCount()))

	j.publish(importer.Event{Type: "step", Phase: "Listing local emails"})
	entries, err := j.Exporter.Store.AllExportEntries(ctx)
	if err != nil {
		j.fail(ctx, "list emails", err)
		return
	}
	total := len(entries)
	_ = j.Exporter.Store.SetImportTotal(ctx, j.ID, total)
	j.publish(importer.Event{Type: "step",
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
		dup := dedup.has(em.SHA256)
		idx := i + 1
		if dup {
			duplicates++
			_ = j.Exporter.Store.IncImportCounters(ctx, j.ID, 1, 1, 0)
			j.publish(importer.Event{Type: "item", Path: key, SHA256: em.SHA256,
				Duplicate: true, Processed: idx, Total: total})
			continue
		}
		if err := j.uploadOne(ctx, client, em, key); err != nil {
			errs++
			_ = j.Exporter.Store.RecordImportError(ctx, j.ID, key, err.Error())
			_ = j.Exporter.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
			j.publish(importer.Event{Type: "item", Path: key, Message: err.Error(),
				Processed: idx, Total: total})
			log.Warn("upload failed",
				slog.String("key", key), slog.String("err", err.Error()))
			continue
		}
		uploaded++
		_ = j.Exporter.Store.IncImportCounters(ctx, j.ID, 1, 0, 0)
		j.publish(importer.Event{Type: "item", Path: key, SHA256: em.SHA256,
			Processed: idx, Total: total})
	}

	finalCtx := context.Background()
	if cancelled {
		j.publish(importer.Event{Type: "step", Phase: "Cancelled"})
		_ = j.Exporter.Store.UpdateImportStatus(finalCtx, j.ID, "error", true)
		j.publish(importer.Event{Type: "error", Phase: "Cancelled",
			Message: "cancelled", Processed: uploaded + duplicates + errs, Total: total})
	} else {
		j.publish(importer.Event{Type: "step", Phase: "Uploading database snapshot"})
		if err := j.uploadDB(ctx, client); err != nil {
			errs++
			_ = j.Exporter.Store.RecordImportError(ctx, j.ID, j.Cfg.Prefix+dbObjectName, err.Error())
			_ = j.Exporter.Store.IncImportCounters(ctx, j.ID, 0, 0, 1)
			log.Warn("db snapshot upload failed", slog.String("err", err.Error()))
		}
		j.publish(importer.Event{Type: "step", Phase: "Finalizing"})
		_ = j.Exporter.Store.UpdateImportStatus(finalCtx, j.ID, "done", true)
		j.publish(importer.Event{Type: "done", Processed: total, Total: total})
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
		Key:         aws.String(j.Cfg.Prefix + dbObjectName),
		Body:        f,
		ContentType: aws.String("application/vnd.sqlite3"),
	})
	return err
}

func (j *S3Job) publish(ev importer.Event) {
	j.Exporter.Hub.Publish(j.ID, ev)
}

func (j *S3Job) fail(ctx context.Context, phase string, cause error) {
	_ = j.Exporter.Store.UpdateImportStatus(ctx, j.ID, "error", true)
	j.publish(importer.Event{Type: "error", Phase: phase, Message: cause.Error()})
	j.logger().Error("s3 export failed",
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
	if len(sha) >= 8 && d.legacy[sha[:8]] {
		return true
	}
	return false
}

func (d *s3Dedup) fullCount() int   { return len(d.full) }
func (d *s3Dedup) legacyCount() int { return len(d.legacy) }

func buildDedup(ctx context.Context, client s3UploadAPI, bucket, prefix string) (*s3Dedup, error) {
	d := &s3Dedup{full: map[string]bool{}, legacy: map[string]bool{}}
	var token *string
	for {
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            nilIfEmpty(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			indexKey(d, aws.ToString(obj.Key), prefix)
		}
		if page.IsTruncated == nil || !*page.IsTruncated {
			break
		}
		token = page.NextContinuationToken
	}
	return d, nil
}

func indexKey(d *s3Dedup, key, prefix string) {
	rel := strings.TrimPrefix(key, prefix)
	if !strings.HasSuffix(strings.ToLower(rel), ".eml") {
		return
	}
	base := rel[:len(rel)-len(".eml")]
	switch {
	case len(base) == 64 && isHexLower(base):
		d.full[base] = true
	case len(base) > 9 && base[8] == '_' && isHexLower(base[:8]):
		d.legacy[base[:8]] = true
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

func nilIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return aws.String(s)
}

func defaultS3UploadClient(ctx context.Context, cfg S3Config) (s3UploadAPI, error) {
	var optFns []func(*config.LoadOptions) error
	if cfg.Region != "" {
		optFns = append(optFns, config.WithRegion(cfg.Region))
	}
	if cfg.AccessKeyID != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return s3.NewFromConfig(awsCfg), nil
}

