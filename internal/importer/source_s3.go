package importer

import (
	"context"
	"fmt"
	"io"

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

// s3API is the subset of *s3.Client used here; lets tests inject a fake.
type s3API interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	GetObject(context.Context, *s3.GetObjectInput, ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type s3Source struct {
	cfg    S3Config
	client s3API // nil until ensureClient
}

func NewS3Source(cfg S3Config) Source { return &s3Source{cfg: cfg} }

func (s *s3Source) Label() string {
	return fmt.Sprintf("s3://%s/%s", s.cfg.Bucket, s.cfg.Prefix)
}

func (s *s3Source) ensureClient(ctx context.Context) error {
	if s.client != nil {
		return nil
	}
	var optFns []func(*config.LoadOptions) error
	if s.cfg.Region != "" {
		optFns = append(optFns, config.WithRegion(s.cfg.Region))
	}
	if s.cfg.AccessKeyID != "" {
		optFns = append(optFns, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				s.cfg.AccessKeyID, s.cfg.SecretAccessKey, s.cfg.SessionToken),
		))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return fmt.Errorf("aws config: %w", err)
	}
	s.client = s3.NewFromConfig(awsCfg)
	return nil
}

func (s *s3Source) Scan(ctx context.Context) ([]Item, error) {
	if err := s.ensureClient(ctx); err != nil {
		return nil, err
	}

	var prefix *string
	if s.cfg.Prefix != "" {
		prefix = aws.String(s.cfg.Prefix)
	}

	var items []Item
	var token *string
	for {
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.cfg.Bucket),
			Prefix:            prefix,
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list objects: %w", err)
		}
		for _, obj := range out.Contents {
			key := aws.ToString(obj.Key)
			if !isEML(key) {
				continue
			}
			k := key
			items = append(items, Item{
				Name: k,
				Open: func(ctx context.Context) (io.ReadCloser, error) {
					return s.getObject(ctx, k)
				},
			})
		}
		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		token = out.NextContinuationToken
	}
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
