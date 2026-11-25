package importer

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

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
	// Endpoint points the client at an S3-compatible service (MinIO,
	// Cloudflare R2, Backblaze B2, Wasabi...) instead of AWS. Empty means AWS.
	Endpoint string
	// PathStyle addresses objects as <endpoint>/<bucket>/<key> rather than
	// <bucket>.<endpoint>/<key>. Most self-hosted services, MinIO included,
	// need it.
	PathStyle bool
}

// defaultCompatRegion is signed into requests to a custom endpoint when no
// region is given. The SDK refuses to sign without one, and S3-compatible
// services either ignore it or treat us-east-1 as their default.
const defaultCompatRegion = "us-east-1"

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
	region := cfg.Region
	if region == "" && cfg.Endpoint != "" {
		region = defaultCompatRegion
	}
	if region != "" {
		optFns = append(optFns, config.WithRegion(region))
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
	return s3.NewFromConfig(awsCfg, s3Options(cfg)), nil
}

// s3Options applies the endpoint settings. With no endpoint the client is
// left exactly as before, so AWS profiles behave as they always have.
func s3Options(cfg S3Config) func(*s3.Options) {
	return func(o *s3.Options) {
		o.UsePathStyle = cfg.PathStyle
		if cfg.Endpoint == "" {
			return
		}
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		// The SDK now adds CRC checksums to every upload by default. Several
		// S3-compatible services reject or mishandle those headers, so only
		// send them where the operation requires one.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	}
}

// NormalizeEndpoint validates a user-entered endpoint and returns it in the
// form the SDK expects. A bare host gets https://; only http and https are
// accepted, and a trailing slash is dropped.
func NormalizeEndpoint(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("invalid endpoint: scheme must be http or https")
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid endpoint: missing host")
	}
	if u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return "", fmt.Errorf("invalid endpoint: only scheme, host, port and path are allowed")
	}
	return strings.TrimRight(u.String(), "/"), nil
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
	label := fmt.Sprintf("s3://%s/%s", c.Bucket, c.Prefix)
	if u, err := url.Parse(c.Endpoint); err == nil && u.Host != "" {
		label += " (" + u.Host + ")"
	}
	return label
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
