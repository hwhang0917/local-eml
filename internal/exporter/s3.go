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
	existing, err := listExistingKeys(ctx, client, j.Cfg.Bucket, j.Cfg.Prefix)
	if err != nil {
		j.fail(ctx, "list destination", err)
		return
	}
	log.Info("destination scan complete", slog.Int("existing", len(existing)))

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
		key := j.Cfg.Prefix + objectName(em.SHA256, em.Filename)
		dup := existing[key]
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

func (j *S3Job) publish(ev importer.Event) {
	j.Exporter.Hub.Publish(j.ID, ev)
}

func (j *S3Job) fail(ctx context.Context, phase string, cause error) {
	_ = j.Exporter.Store.UpdateImportStatus(ctx, j.ID, "error", true)
	j.publish(importer.Event{Type: "error", Phase: phase, Message: cause.Error()})
	j.logger().Error("s3 export failed",
		slog.String("phase", phase), slog.String("err", cause.Error()))
}

func listExistingKeys(ctx context.Context, client s3UploadAPI, bucket, prefix string) (map[string]bool, error) {
	out := map[string]bool{}
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
			out[aws.ToString(obj.Key)] = true
		}
		if page.IsTruncated == nil || !*page.IsTruncated {
			break
		}
		token = page.NextContinuationToken
	}
	return out, nil
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

