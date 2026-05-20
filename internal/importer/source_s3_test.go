package importer

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type fakeS3 struct {
	pages   []*s3.ListObjectsV2Output
	objects map[string]string
	listN   int
}

func (f *fakeS3) ListObjectsV2(_ context.Context, _ *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	out := f.pages[f.listN]
	f.listN++
	return out, nil
}

func (f *fakeS3) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	body := f.objects[aws.ToString(in.Key)]
	return &s3.GetObjectOutput{Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestS3SourceScanPaginatesAndFiltersEML(t *testing.T) {
	truthy := true
	fake := &fakeS3{
		objects: map[string]string{"a.eml": "AAA", "b.eml": "BBB"},
		pages: []*s3.ListObjectsV2Output{
			{
				IsTruncated:           &truthy,
				NextContinuationToken: aws.String("tok"),
				Contents: []types.Object{
					{Key: aws.String("a.eml")},
					{Key: aws.String("skip.txt")},
				},
			},
			{
				Contents: []types.Object{{Key: aws.String("b.eml")}},
			},
		},
	}

	src := &s3Source{cfg: S3Config{Bucket: "bkt"}, client: fake}
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 eml items across 2 pages, got %d", len(items))
	}

	rc, err := items[0].Open(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, _ := io.ReadAll(rc)
	if string(b) != "AAA" {
		t.Errorf("body = %q, want AAA", string(b))
	}
}

func TestS3SourceLabel(t *testing.T) {
	src := &s3Source{cfg: S3Config{Bucket: "bkt", Prefix: "mail/"}}
	if got := src.Label(); got != "s3://bkt/mail/" {
		t.Errorf("label = %q", got)
	}
}
