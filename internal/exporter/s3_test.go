package exporter

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/hwhang0917/local-eml/internal/importer"
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

// fakeS3 implements s3UploadAPI for hermetic tests.
type fakeS3 struct {
	mu       sync.Mutex
	existing map[string]bool
	put      []string // ordered list of keys uploaded by the job
}

func newFakeS3(existing ...string) *fakeS3 {
	f := &fakeS3{existing: map[string]bool{}}
	for _, k := range existing {
		f.existing[k] = true
	}
	return f
}

func (f *fakeS3) ListObjectsV2(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := aws.ToString(in.Prefix)
	var contents []s3types.Object
	keys := make([]string, 0, len(f.existing))
	for k := range f.existing {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if prefix == "" || len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			contents = append(contents, s3types.Object{Key: aws.String(k)})
		}
	}
	return &s3.ListObjectsV2Output{Contents: contents}, nil
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	k := aws.ToString(in.Key)
	f.put = append(f.put, k)
	f.existing[k] = true
	return &s3.PutObjectOutput{}, nil
}

func newExporterFixture(t *testing.T, entries []store.EmailRow) (*Exporter, *importer.Hub, string) {
	t.Helper()
	tmp := t.TempDir()
	p := paths.Paths{
		Base: tmp,
		EML:  filepath.Join(tmp, "eml"),
		DB:   filepath.Join(tmp, "db"),
		Logs: filepath.Join(tmp, "logs"),
		Keys: filepath.Join(tmp, "keys"),
	}
	if err := p.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(context.Background(), p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	for _, e := range entries {
		if _, err := st.InsertEmail(context.Background(), e); err != nil {
			t.Fatalf("insert: %v", err)
		}
		if err := os.WriteFile(p.BlobFor(e.SHA256), []byte("dummy"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	hub := importer.NewHub()
	exp := &Exporter{Store: st, Paths: p, Hub: hub}
	importID := "exp-test"
	if err := st.CreateImport(context.Background(), store.Import{
		ID: importID, SourceKind: "s3-export", Status: "queued",
	}); err != nil {
		t.Fatal(err)
	}
	return exp, hub, importID
}

func runWithFake(t *testing.T, exp *Exporter, importID, prefix string, fake *fakeS3) (uploaded []string, dups, errs int) {
	t.Helper()
	job := exp.NewS3Job(importID, S3Config{Bucket: "b", Prefix: prefix})
	job.newClient = func(context.Context, S3Config) (s3UploadAPI, error) { return fake, nil }

	// Drain SSE so Publish doesn't block the job goroutine on a full buffer.
	ch, cancel := exp.Hub.Subscribe(importID)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
		}
	}()

	job.Run(context.Background())
	cancel()
	<-done

	imp, err := exp.Store.GetImport(context.Background(), importID)
	if err != nil {
		t.Fatal(err)
	}
	return fake.put, imp.Duplicates, imp.Errors
}

func entry(shaSuffix, filename string) store.EmailRow {
	sha := "0000000000000000000000000000000000000000000000000000000000000000"
	sha = sha[:len(sha)-len(shaSuffix)] + shaSuffix
	return store.EmailRow{
		Email: store.Email{
			SHA256:   sha,
			Filename: filename,
			Subject:  "s",
			FromAddr: "f@example",
		},
		BodyText: "body",
	}
}

func TestS3Export_FreshBucket_UploadsSHANamedKeys(t *testing.T) {
	rows := []store.EmailRow{entry("aabbccdd", "mail/2024/foo.eml")}
	exp, _, id := newExporterFixture(t, rows)
	fake := newFakeS3()

	uploaded, dups, errs := runWithFake(t, exp, id, "out/", fake)

	if errs != 0 {
		t.Fatalf("errors=%d, want 0", errs)
	}
	if dups != 0 {
		t.Errorf("duplicates=%d, want 0", dups)
	}
	if len(uploaded) != 1 {
		t.Fatalf("uploaded=%v, want 1", uploaded)
	}
	wantKey := "out/" + rows[0].SHA256 + ".eml"
	if uploaded[0] != wantKey {
		t.Errorf("uploaded[0]=%q, want %q", uploaded[0], wantKey)
	}
}

func TestS3Export_ReExport_SkipsByContentAddressedKey(t *testing.T) {
	rows := []store.EmailRow{entry("11223344", "x.eml")}
	exp, _, id := newExporterFixture(t, rows)
	prefix := "out/"
	existing := prefix + rows[0].SHA256 + ".eml"
	fake := newFakeS3(existing)

	uploaded, dups, errs := runWithFake(t, exp, id, prefix, fake)

	if errs != 0 {
		t.Fatalf("errors=%d, want 0", errs)
	}
	if len(uploaded) != 0 {
		t.Errorf("uploaded=%v, want []", uploaded)
	}
	if dups != 1 {
		t.Errorf("duplicates=%d, want 1", dups)
	}
}

func TestS3Export_LegacyBuggyKey_RecognizedAsDuplicate(t *testing.T) {
	// User was bitten by the pre-fix code which wrote <sha8>_<basename>.eml.
	// A re-run with the fixed code must NOT pile another <sha>.eml copy on top.
	rows := []store.EmailRow{entry("deadbeef", "anything/foo.eml")}
	exp, _, id := newExporterFixture(t, rows)
	prefix := "out/"
	short := rows[0].SHA256[:8]
	legacy := prefix + short + "_foo.eml"
	fake := newFakeS3(legacy)

	uploaded, dups, errs := runWithFake(t, exp, id, prefix, fake)

	if errs != 0 {
		t.Fatalf("errors=%d, want 0", errs)
	}
	if len(uploaded) != 0 {
		t.Errorf("uploaded=%v, want [] (legacy key should dedup)", uploaded)
	}
	if dups != 1 {
		t.Errorf("duplicates=%d, want 1", dups)
	}
}

func TestS3Export_DifferentPrefixDoesNotDedup(t *testing.T) {
	// listObjects is scoped to the export prefix; objects outside it are
	// invisible. A bucket key matching the SHA but living under a different
	// prefix should NOT cause us to skip — different destination, different
	// concern.
	rows := []store.EmailRow{entry("12121212", "x.eml")}
	exp, _, id := newExporterFixture(t, rows)
	existing := "elsewhere/" + rows[0].SHA256 + ".eml"
	fake := newFakeS3(existing)

	uploaded, _, _ := runWithFake(t, exp, id, "out/", fake)

	if len(uploaded) != 1 {
		t.Fatalf("uploaded=%v, want 1 (different prefix should not dedup)", uploaded)
	}
}
