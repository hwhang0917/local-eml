package importer

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestNormalizeEndpoint(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"", "", true},
		{"  ", "", true},
		{"http://localhost:9000", "http://localhost:9000", true},
		{"http://localhost:9000/", "http://localhost:9000", true},
		{"s3.us-west-004.backblazeb2.com", "https://s3.us-west-004.backblazeb2.com", true},
		{"https://acct.r2.cloudflarestorage.com", "https://acct.r2.cloudflarestorage.com", true},
		{"ftp://example.com", "", false},
		{"http://", "", false},
		{"https://user:pw@example.com", "", false},
		{"https://example.com/?x=1", "", false},
	}
	for _, c := range cases {
		got, err := NormalizeEndpoint(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("NormalizeEndpoint(%q) = %q, %v; want %q ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

func TestS3ConfigLabelShowsEndpointHost(t *testing.T) {
	cfg := S3Config{Bucket: "bkt", Prefix: "mail/", Endpoint: "http://localhost:9000"}
	if got := cfg.Label(); got != "s3://bkt/mail/ (localhost:9000)" {
		t.Fatalf("label=%q", got)
	}
}

// With no endpoint the client must look exactly like it did before
// S3-compatible support: AWS addressing and the SDK's checksum defaults.
func TestS3OptionsLeaveAWSUntouched(t *testing.T) {
	var o s3.Options
	s3Options(S3Config{Bucket: "b"})(&o)
	if o.BaseEndpoint != nil || o.UsePathStyle || o.RequestChecksumCalculation != 0 {
		t.Fatalf("aws options changed: endpoint=%v pathStyle=%v checksum=%v",
			o.BaseEndpoint, o.UsePathStyle, o.RequestChecksumCalculation)
	}
}

// A real client against a local HTTP server: requests must go to the custom
// endpoint, path-style, signed with the fallback region.
func TestNewS3ClientCustomEndpointPathStyle(t *testing.T) {
	var gotHost, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotPath, gotAuth = r.Host, r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><Name>bkt</Name><IsTruncated>false</IsTruncated>
<Contents><Key>a.eml</Key></Contents></ListBucketResult>`)
	}))
	defer srv.Close()

	client, err := NewS3Client(context.Background(), S3Config{
		AccessKeyID: "minioadmin", SecretAccessKey: "minioadmin",
		Bucket: "bkt", Endpoint: srv.URL, PathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	if err := ListKeys(context.Background(), client, "bkt", "", func(k string) { keys = append(keys, k) }); err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || keys[0] != "a.eml" {
		t.Fatalf("keys=%v", keys)
	}
	if gotHost != strings.TrimPrefix(srv.URL, "http://") || gotPath != "/bkt" {
		t.Errorf("request went to host=%q path=%q, want path-style on the test server", gotHost, gotPath)
	}
	if !strings.Contains(gotAuth, "/us-east-1/s3/") {
		t.Errorf("authorization %q not signed for the fallback region", gotAuth)
	}
}
