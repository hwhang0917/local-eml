package importer

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Config struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
	Bucket          string
	Prefix          string
}

// S3Lister is the one call key listing needs; *s3.Client and test fakes
// satisfy it.
type S3Lister interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// s3API is the subset of *s3.Client used here; lets tests inject a fake.
type s3API interface {
	S3Lister
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

// NewS3Client builds a client for cfg. Static keys win when given; otherwise
// the SDK's usual credential chain (env, shared config, instance role) applies.
func NewS3Client(ctx context.Context, cfg S3Config) (*s3.Client, error) {
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

// ListKeys calls fn with every object key under bucket/prefix, following
// continuation tokens until the listing is exhausted.
func ListKeys(ctx context.Context, client S3Lister, bucket, prefix string, fn func(key string)) error {
	var prefixArg *string
	if prefix != "" {
		prefixArg = aws.String(prefix)
	}
	var token *string
	for {
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            prefixArg,
			ContinuationToken: token,
		})
		if err != nil {
			return err
		}
		for _, obj := range page.Contents {
			fn(aws.ToString(obj.Key))
		}
		if page.IsTruncated == nil || !*page.IsTruncated {
			return nil
		}
		token = page.NextContinuationToken
	}
}

type s3Source struct {
	cfg    S3Config
	client s3API // nil until ensureClient
}

func NewS3Source(cfg S3Config) Source { return &s3Source{cfg: cfg} }

// Label names the destination the way the job list shows it.
func (c S3Config) Label() string {
	return fmt.Sprintf("s3://%s/%s", c.Bucket, c.Prefix)
}

func (s *s3Source) Label() string { return s.cfg.Label() }

func (s *s3Source) ensureClient(ctx context.Context) error {
	if s.client != nil {
		return nil
	}
	client, err := NewS3Client(ctx, s.cfg)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func (s *s3Source) Scan(ctx context.Context) ([]Item, error) {
	if err := s.ensureClient(ctx); err != nil {
		return nil, err
	}

	log := slog.Default().With(
		slog.String("bucket", s.cfg.Bucket), slog.String("prefix", s.cfg.Prefix))

	var items []Item
	scanned := 0
	err := ListKeys(ctx, s.client, s.cfg.Bucket, s.cfg.Prefix, func(key string) {
		scanned++
		if !IsEML(key) {
			return
		}
		items = append(items, Item{
			Name: key,
			Open: func(ctx context.Context) (io.ReadCloser, error) {
				return s.getObject(ctx, key)
			},
		})
	})
	if err != nil {
		log.Error("s3 list objects failed", slog.String("err", err.Error()))
		return nil, fmt.Errorf("list objects: %w", err)
	}
	log.Info("s3 scan complete", slog.Int("objects", scanned), slog.Int("eml", len(items)))
	return items, nil
}

func (s *s3Source) getObject(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", key, err)
	}
	return out.Body, nil
}
