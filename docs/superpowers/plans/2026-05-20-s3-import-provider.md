# S3 Import Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an AWS S3 import provider to 가져오기 that recursively imports `*.eml` objects from a bucket, behind a pluggable `Source` interface that also absorbs the existing local file/dir/zip providers.

**Architecture:** Introduce `importer.Source` (`Label()` + `Scan() []Item`, each `Item` a lazy `Open` closure). One generic `Job.RunSource` drives any source through the unchanged dedup/parse/store pipeline and SSE event stream. Local upload becomes `localFilesSource`/`zipSource`; the new `s3Source` lists+gets objects via `aws-sdk-go-v2`. A JSON endpoint `POST /api/imports/s3` accepts optional credentials (falling back to the SDK default chain), bucket, and prefix.

**Tech Stack:** Go 1.25, `aws-sdk-go-v2` (`config`, `credentials`, `service/s3`), chi, SSE; Vue 3 + Vite + TypeScript + Tailwind, vue-i18n.

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/importer/source.go` (create) | `Source` interface + `Item` type |
| `internal/importer/source_local.go` (create) | `localFilesSource`, `zipSource` + constructors |
| `internal/importer/source_s3.go` (create) | `S3Config`, `s3Source`, `s3API` seam |
| `internal/importer/job.go` (modify) | Replace `RunFile/RunDir/RunZip` with `RunSource` + `processItem` |
| `internal/importer/source_test.go` (create) | local/zip source + `RunSource` driver tests |
| `internal/importer/source_s3_test.go` (create) | `s3Source.Scan` against a fake `s3API` |
| `internal/server/handlers_imports.go` (modify) | Build sources, generalize `runJob`, add `handleImportS3` |
| `internal/server/router.go` (modify) | Register `POST /api/imports/s3` |
| `web/src/lib/api.ts` (modify) | `uploadS3(cfg)` |
| `web/src/composables/useImports.ts` (modify) | `startS3Import(cfg)` |
| `web/src/pages/ImportPage.vue` (modify) | Provider toggle + S3 form |
| `web/src/locales/{en,ko}.json` (modify) | New `import.*` keys |

**Commands used throughout:**
- Go build: `CGO_ENABLED=0 go build ./...`
- Go tests: `CGO_ENABLED=1 go test ./internal/importer/... -race -count=1` (race needs cgo, per Makefile)
- Web type-check: `cd web && npm run type-check`

(Per project convention the maintainer runs final build/serve; the executing agent runs the test/build commands below to verify each task.)

---

## Task 1: Add AWS SDK v2 dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the three modules at their verified-latest versions**

Run:
```bash
go get github.com/aws/aws-sdk-go-v2/config@v1.32.17
go get github.com/aws/aws-sdk-go-v2/credentials@v1.19.16
go get github.com/aws/aws-sdk-go-v2/service/s3@v1.101.0
go mod tidy
```
(These versions were confirmed to exist on proxy.golang.org. `go mod tidy` pulls the transitive `github.com/aws/aws-sdk-go-v2` core + smithy-go.)

- [ ] **Step 2: Verify it builds**

Run: `CGO_ENABLED=0 go build ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: add aws-sdk-go-v2 (config, credentials, s3)"
```

---

## Task 2: Source interface

**Files:**
- Create: `internal/importer/source.go`

- [ ] **Step 1: Write the interface**

```go
package importer

import (
	"context"
	"io"
)

// Item is one importable object. Open is lazy so bodies stream one at a time.
type Item struct {
	Name string
	Open func(ctx context.Context) (io.ReadCloser, error)
}

// Source enumerates .eml items from a provider (local upload, zip, S3, …).
type Source interface {
	// Label is a short human description for the "Scanning <label>" phase.
	Label() string
	// Scan returns candidate items already filtered to .eml.
	Scan(ctx context.Context) ([]Item, error)
}
```

- [ ] **Step 2: Verify it builds**

Run: `CGO_ENABLED=0 go build ./internal/importer/...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/importer/source.go
git commit -m "feat(importer): add Source interface and Item"
```

---

## Task 3: Local sources (file/dir/zip)

**Files:**
- Create: `internal/importer/source_local.go`
- Test: `internal/importer/source_test.go` (created here, extended in Task 4)

- [ ] **Step 1: Write the failing tests**

Create `internal/importer/source_test.go`:
```go
package importer

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func readItem(t *testing.T, it Item) string {
	t.Helper()
	rc, err := it.Open(context.Background())
	if err != nil {
		t.Fatalf("open %s: %v", it.Name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %s: %v", it.Name, err)
	}
	return string(b)
}

func TestLocalFilesSourceFiltersEMLWhenRequested(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "a.eml")
	bad := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(good, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte("nope"), 0o600); err != nil {
		t.Fatal(err)
	}

	src := NewLocalSource("2 files", []string{good, bad}, []string{"a.eml", "notes.txt"}, true)
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 eml item, got %d", len(items))
	}
	if items[0].Name != "a.eml" {
		t.Errorf("name = %q", items[0].Name)
	}
	if got := readItem(t, items[0]); got != "hello" {
		t.Errorf("body = %q", got)
	}
}

func TestLocalFilesSourceKeepsNonEMLWhenFilterOff(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "single")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := NewLocalSource("upload", []string{p}, []string{"single.eml"}, false)
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1, got %d", len(items))
	}
}

func TestZipSourceScansEMLEntries(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "c.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range map[string]string{
		"inbox/one.eml": "ONE",
		"inbox/two.eml": "TWO",
		"readme.txt":    "skip",
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	src := NewZipSource(zipPath)
	defer src.Close()
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 eml items, got %d", len(items))
	}
	for _, it := range items {
		if filepath.Ext(it.Name) != ".eml" {
			t.Errorf("unexpected entry %q", it.Name)
		}
		if readItem(t, it) == "" {
			t.Errorf("empty body for %q", it.Name)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -run 'LocalFiles|ZipSource' -count=1`
Expected: FAIL — `undefined: NewLocalSource` / `NewZipSource`.

- [ ] **Step 3: Implement the local sources**

Create `internal/importer/source_local.go`:
```go
package importer

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// localFilesSource reads files already on disk (multipart temp files).
// filterEML drops non-.eml names (used for directory uploads); single-file
// uploads pass filterEML=false so the chosen file is always imported.
type localFilesSource struct {
	label     string
	paths     []string
	names     []string
	filterEML bool
}

func NewLocalSource(label string, paths, names []string, filterEML bool) Source {
	return &localFilesSource{label: label, paths: paths, names: names, filterEML: filterEML}
}

func (s *localFilesSource) Label() string { return s.label }

func (s *localFilesSource) Scan(_ context.Context) ([]Item, error) {
	items := make([]Item, 0, len(s.names))
	for i, name := range s.names {
		if s.filterEML && !isEML(name) {
			continue
		}
		p := s.paths[i]
		items = append(items, Item{
			Name: name,
			Open: func(context.Context) (io.ReadCloser, error) { return os.Open(p) },
		})
	}
	return items, nil
}

// zipSource streams .eml entries out of a zip archive. The archive is opened
// in Scan and held until Close (called by the driver via io.Closer).
type zipSource struct {
	path string
	zr   *zip.ReadCloser
}

func NewZipSource(path string) *zipSource { return &zipSource{path: path} }

func (s *zipSource) Label() string { return "zip archive" }

func (s *zipSource) Scan(_ context.Context) ([]Item, error) {
	zr, err := zip.OpenReader(s.path)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	s.zr = zr
	var items []Item
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !isEML(f.Name) {
			continue
		}
		ze := f
		items = append(items, Item{
			Name: filepath.Base(ze.Name),
			Open: func(context.Context) (io.ReadCloser, error) { return ze.Open() },
		})
	}
	return items, nil
}

func (s *zipSource) Close() error {
	if s.zr != nil {
		return s.zr.Close()
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -run 'LocalFiles|ZipSource' -count=1`
Expected: PASS (ok).

- [ ] **Step 5: Commit**

```bash
git add internal/importer/source_local.go internal/importer/source_test.go
git commit -m "feat(importer): local file/dir/zip Source implementations"
```

---

## Task 4: Generic RunSource driver

**Files:**
- Modify: `internal/importer/job.go`
- Test: `internal/importer/source_test.go` (append `RunSource` test)

- [ ] **Step 1: Append the failing driver test**

Add to `internal/importer/source_test.go`:
```go
import (
	"errors"
	"strings"
	// (keep existing imports; also add the ones below)
)

// minimalEML is a parseable RFC822 message.
const minimalEML = "From: a@example.com\r\nTo: b@example.com\r\nSubject: hi\r\n\r\nbody\r\n"

type stubSource struct {
	label string
	items []Item
}

func (s stubSource) Label() string                          { return s.label }
func (s stubSource) Scan(context.Context) ([]Item, error)   { return s.items, nil }

func TestRunSourceImportsAndContinuesPastErrors(t *testing.T) {
	st := newTestStore(t)
	im := &Importer{Store: st, Paths: newTestPaths(t)}
	hub := NewHub()

	importID := "imp1"
	if err := st.CreateImport(context.Background(), importStub(importID)); err != nil {
		t.Fatal(err)
	}

	src := stubSource{
		label: "stub",
		items: []Item{
			{Name: "good.eml", Open: func(context.Context) (io.ReadCloser, error) {
				return io.NopCloser(strings.NewReader(minimalEML)), nil
			}},
			{Name: "broken.eml", Open: func(context.Context) (io.ReadCloser, error) {
				return nil, errors.New("boom")
			}},
		},
	}

	job := &Job{Importer: im, Hub: hub, Store: st, ID: importID}
	job.RunSource(context.Background(), src)

	imp, err := st.GetImport(context.Background(), importID)
	if err != nil {
		t.Fatal(err)
	}
	if imp.Status != "done" {
		t.Errorf("status = %q, want done", imp.Status)
	}
	if imp.Total != 2 {
		t.Errorf("total = %d, want 2", imp.Total)
	}
	if imp.Processed != 2 {
		t.Errorf("processed = %d, want 2", imp.Processed)
	}
	if imp.Errors != 1 {
		t.Errorf("errors = %d, want 1", imp.Errors)
	}
}
```

Also add these helpers to `internal/importer/source_test.go` (real temp store + paths; no network, no mocks of the DB):
```go
import (
	"github.com/hwhang0917/local-eml/internal/paths"
	"github.com/hwhang0917/local-eml/internal/store"
)

func newTestPaths(t *testing.T) paths.Paths {
	t.Helper()
	base := t.TempDir()
	p := paths.Paths{
		Base: base,
		EML:  filepath.Join(base, "eml"),
		DB:   filepath.Join(base, "db"),
		Logs: filepath.Join(base, "logs"),
	}
	if err := p.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	return p
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	p := newTestPaths(t)
	st, err := store.Open(context.Background(), p.DBFile())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func importStub(id string) store.Import {
	return store.Import{ID: id, SourceKind: "stub", SourceName: "stub", Status: "queued"}
}
```
> Note: confirm `store.Store` exposes `Close()` — if the method differs, adjust the cleanup. `store.Open(ctx, dsn)` and `GetImport(ctx, id)` are existing signatures.

- [ ] **Step 2: Run the test to verify it fails**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -run RunSource -count=1`
Expected: FAIL — `job.RunSource undefined`.

- [ ] **Step 3: Replace the Run\* methods with RunSource**

In `internal/importer/job.go`, delete `RunFile`, `RunDir`, `RunZip`, `processOne`, and `processZipEntry`. Replace the imports block and add the new driver. The file becomes:
```go
package importer

import (
	"context"
	"fmt"
	"io"

	"github.com/hwhang0917/local-eml/internal/store"
)

type Job struct {
	Importer *Importer
	Hub      *Hub
	Store    *store.Store
	ID       string
}

func (j *Job) RunSource(ctx context.Context, src Source) {
	defer j.Hub.Close(j.ID)
	if c, ok := src.(io.Closer); ok {
		defer c.Close()
	}

	j.publish(Event{Type: "step", Phase: "Scanning " + src.Label()})
	items, err := src.Scan(ctx)
	if err != nil {
		_ = j.Store.UpdateImportStatus(ctx, j.ID, "error", true)
		j.publish(Event{Type: "error", Message: err.Error()})
		return
	}

	total := len(items)
	_ = j.Store.SetImportTotal(ctx, j.ID, total)
	j.publish(Event{Type: "step", Phase: fmt.Sprintf("Importing %d emails", total), Total: total})

	for i, it := range items {
		if ctxDone(ctx) {
			break
		}
		j.processItem(ctx, it, i+1, total)
	}

	j.publish(Event{Type: "step", Phase: "Finalizing"})
	_ = j.Store.UpdateImportStatus(ctx, j.ID, "done", true)
	j.publish(Event{Type: "done", Processed: total, Total: total})
}

func (j *Job) processItem(ctx context.Context, it Item, idx, total int) {
	rc, err := it.Open(ctx)
	if err != nil {
		j.recordItemError(ctx, it.Name, err, idx, total)
		return
	}
	defer rc.Close()

	res, err := j.Importer.ImportReader(ctx, rc, it.Name)
	if err != nil {
		j.recordItemError(ctx, it.Name, err, idx, total)
		return
	}
	dup := 0
	if res.Duplicate {
		dup = 1
	}
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, dup, 0)
	j.publish(Event{Type: "item", Path: it.Name, SHA256: res.SHA256,
		Duplicate: res.Duplicate, Processed: idx, Total: total})
}

func (j *Job) recordItemError(ctx context.Context, name string, cause error, idx, total int) {
	_ = j.Store.RecordImportError(ctx, j.ID, name, cause.Error())
	_ = j.Store.IncImportCounters(ctx, j.ID, 1, 0, 1)
	j.publish(Event{Type: "item", Path: name, Message: cause.Error(),
		Processed: idx, Total: total})
}

func (j *Job) publish(ev Event) {
	j.Hub.Publish(j.ID, ev)
}

func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func isEML(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".eml")
}
```
> `isEML` uses `strings`; keep the `strings` import. Final import list for job.go: `context`, `fmt`, `io`, `strings`, and the `store` package.

Corrected import block for `internal/importer/job.go`:
```go
import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hwhang0917/local-eml/internal/store"
)
```

> This breaks `internal/server/handlers_imports.go`, which still calls `RunFile/RunDir/RunZip`. That package is fixed in Task 6. The `internal/importer` package itself compiles; verify with the package-scoped build below before moving on.

- [ ] **Step 4: Verify the importer package builds and the driver test passes**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -count=1`
Expected: PASS (all importer tests, including RunSource and the Task 3 source tests).

- [ ] **Step 5: Commit**

```bash
git add internal/importer/job.go internal/importer/source_test.go
git commit -m "refactor(importer): drive imports through Source via RunSource"
```

---

## Task 5: S3 source

**Files:**
- Create: `internal/importer/source_s3.go`
- Test: `internal/importer/source_s3_test.go`

- [ ] **Step 1: Write the failing test (fake s3API, no network)**

Create `internal/importer/source_s3_test.go`:
```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -run S3Source -count=1`
Expected: FAIL — `undefined: s3Source` / `S3Config`.

- [ ] **Step 3: Implement the S3 source**

Create `internal/importer/source_s3.go`:
```go
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
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -run S3Source -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full importer package**

Run: `CGO_ENABLED=1 go test ./internal/importer/... -count=1`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/importer/source_s3.go internal/importer/source_s3_test.go
git commit -m "feat(importer): S3 Source with credential fallback"
```

---

## Task 6: Wire handlers + route

**Files:**
- Modify: `internal/server/handlers_imports.go`
- Modify: `internal/server/router.go`

- [ ] **Step 1: Generalize runJob and build sources in the upload handler**

In `internal/server/handlers_imports.go`, replace the body of `handleImportUpload` from the `kind := detectKind(files)` line onward, and replace `runJob` entirely:

Replace this section of `handleImportUpload`:
```go
	kind := detectKind(files)
	importID := newImportID()
	sourceName := files[0].Filename
	if len(files) > 1 {
		sourceName = fmt.Sprintf("(%d files)", len(files))
	}

	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: kind,
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		removeAll(paths)
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	go s.runJob(importID, kind, paths, names)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      kind,
	})
}
```
with:
```go
	kind := detectKind(files)
	importID := newImportID()
	sourceName := files[0].Filename
	if len(files) > 1 {
		sourceName = fmt.Sprintf("(%d files)", len(files))
	}

	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: kind,
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		removeAll(paths)
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var src importer.Source
	switch kind {
	case "zip":
		src = importer.NewZipSource(paths[0])
	case "file":
		src = importer.NewLocalSource(sourceName, paths, names, false)
	default: // "dir"
		src = importer.NewLocalSource(sourceName, paths, names, true)
	}

	go s.runJob(importID, src, func() { removeAll(paths) })

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      kind,
	})
}
```

Replace `runJob`:
```go
func (s *Server) runJob(importID string, src importer.Source, cleanup func()) {
	defer cleanup()
	ctx := context.Background()
	_ = s.Store.UpdateImportStatus(ctx, importID, "running", false)

	job := &importer.Job{
		Importer: s.Importer,
		Hub:      s.Hub,
		Store:    s.Store,
		ID:       importID,
	}

	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("import job panic", "import_id", importID, "panic", rec)
			_ = s.Store.UpdateImportStatus(ctx, importID, "error", true)
			s.Hub.Publish(importID, importer.Event{Type: "error", Message: fmt.Sprint(rec)})
			s.Hub.Close(importID)
		}
	}()

	job.RunSource(ctx, src)
}
```

- [ ] **Step 2: Add the S3 handler**

Append to `internal/server/handlers_imports.go`:
```go
func (s *Server) handleImportS3(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccessKeyID     string `json:"accessKeyId"`
		SecretAccessKey string `json:"secretAccessKey"`
		SessionToken    string `json:"sessionToken"`
		Region          string `json:"region"`
		Bucket          string `json:"bucket"`
		Prefix          string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "decode body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Bucket) == "" {
		http.Error(w, "bucket is required", http.StatusBadRequest)
		return
	}

	importID := newImportID()
	sourceName := fmt.Sprintf("s3://%s/%s", body.Bucket, body.Prefix)
	if err := s.Store.CreateImport(r.Context(), store.Import{
		ID:         importID,
		SourceKind: "s3",
		SourceName: sourceName,
		Status:     "queued",
	}); err != nil {
		http.Error(w, "create import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	src := importer.NewS3Source(importer.S3Config{
		AccessKeyID:     body.AccessKeyID,
		SecretAccessKey: body.SecretAccessKey,
		SessionToken:    body.SessionToken,
		Region:          body.Region,
		Bucket:          body.Bucket,
		Prefix:          body.Prefix,
	})

	go s.runJob(importID, src, func() {})

	writeJSON(w, http.StatusAccepted, map[string]any{
		"import_id": importID,
		"kind":      "s3",
	})
}
```
> `json`, `strings`, `fmt`, `importer`, and `store` are already imported in this file.

- [ ] **Step 3: Register the route**

In `internal/server/router.go`, add after the `api.Post("/imports", ...)` line:
```go
		api.Post("/imports/s3", s.handleImportS3)
```

- [ ] **Step 4: Build and run all Go tests**

Run: `CGO_ENABLED=0 go build ./... && CGO_ENABLED=1 go test ./... -race -count=1`
Expected: build clean; all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/handlers_imports.go internal/server/router.go
git commit -m "feat(server): POST /api/imports/s3 + Source-based runJob"
```

---

## Task 7: Frontend API client

**Files:**
- Modify: `web/src/lib/api.ts`

- [ ] **Step 1: Add the S3 config type and uploadS3 method**

In `web/src/lib/api.ts`, add this interface near the other interfaces (e.g. after `ImportEvent`):
```ts
export interface S3ImportConfig {
  accessKeyId?: string
  secretAccessKey?: string
  sessionToken?: string
  region?: string
  bucket: string
  prefix?: string
}
```

Add this method to the `api` object, right after `upload(...)`:
```ts
  async uploadS3(cfg: S3ImportConfig): Promise<{ import_id: string; kind: string }> {
    const res = await fetch(`${BASE}/api/imports/s3`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    })
    return jsonOrThrow(res)
  },
```

- [ ] **Step 2: Type-check**

Run: `cd web && npm run type-check`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/api.ts
git commit -m "feat(web): api.uploadS3 client"
```

---

## Task 8: Frontend composable

**Files:**
- Modify: `web/src/composables/useImports.ts`

- [ ] **Step 1: Add startS3Import**

In `web/src/composables/useImports.ts`, update the import line and the `useImports` return.

Change the api import line:
```ts
import { api, type ImportEvent, type S3ImportConfig } from '@/lib/api'
```

Replace the `useImports` function with:
```ts
export function useImports() {
  async function startImport(files: File[]) {
    const { import_id, kind } = await api.upload(files)
    const run = reactive<ImportRun>({
      id: import_id,
      kind,
      phase: t('import.queued'),
      current: '',
      total: kind === 'zip' ? 0 : files.length,
      processed: 0,
      duplicates: 0,
      errors: 0,
      done: false,
      log: [],
    })
    runs.value.unshift(run)
    followProgress(run)
  }

  async function startS3Import(cfg: S3ImportConfig) {
    const { import_id, kind } = await api.uploadS3(cfg)
    const run = reactive<ImportRun>({
      id: import_id,
      kind,
      phase: t('import.queued'),
      current: '',
      total: 0,
      processed: 0,
      duplicates: 0,
      errors: 0,
      done: false,
      log: [],
    })
    runs.value.unshift(run)
    followProgress(run)
  }

  return { runs, startImport, startS3Import }
}
```

- [ ] **Step 2: Type-check**

Run: `cd web && npm run type-check`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/composables/useImports.ts
git commit -m "feat(web): startS3Import in useImports"
```

---

## Task 9: Import page provider UI

**Files:**
- Modify: `web/src/pages/ImportPage.vue`

- [ ] **Step 1: Replace ImportPage.vue with the provider-aware version**

Overwrite `web/src/pages/ImportPage.vue` with:
```vue
<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { cn } from '@/lib/utils'
import { formatBytes } from '@/lib/format'
import { useImports } from '@/composables/useImports'
import type { S3ImportConfig } from '@/lib/api'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const { runs, startImport, startS3Import } = useImports()

type Provider = 'local' | 's3'
const provider = ref<Provider>('local')

// --- local upload (unchanged behavior) ---
const stagedFiles = ref<File[]>([])
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dirInput = ref<HTMLInputElement | null>(null)

const stagedTotal = computed(() =>
  stagedFiles.value.reduce((s, f) => s + f.size, 0),
)

const stagedSummary = computed(() => {
  const total = formatBytes(stagedTotal.value)
  return stagedFiles.value.length === 1
    ? t('import.confirm_summary_one', { total })
    : t('import.confirm_summary_many', { count: stagedFiles.value.length, total })
})

function stageFiles(files: File[]) {
  if (files.length === 0) return
  stagedFiles.value = files
}

function clearStaged() {
  stagedFiles.value = []
}

async function confirmUpload() {
  if (stagedFiles.value.length === 0) return
  const files = stagedFiles.value
  stagedFiles.value = []
  await startImport(files)
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (!files) return
  stageFiles(Array.from(files))
}

function onFilePicked(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  stageFiles(Array.from(input.files))
  input.value = ''
}

const dropzoneClass = computed(() =>
  cn(
    'border-2 border-dashed text-center p-12 transition-colors',
    dragOver.value ? 'border-primary bg-accent/50' : 'border-border',
  ),
)

// --- S3 provider ---
const s3 = ref<S3ImportConfig>({
  accessKeyId: '',
  secretAccessKey: '',
  sessionToken: '',
  region: '',
  bucket: '',
  prefix: '',
})
const s3Confirming = ref(false)

const s3Valid = computed(() => s3.value.bucket.trim().length > 0)

const s3CredsLabel = computed(() =>
  s3.value.accessKeyId?.trim() ? t('import.s3_creds_form') : t('import.s3_creds_system'),
)

function reviewS3() {
  if (s3Valid.value) s3Confirming.value = true
}

function cancelS3() {
  s3Confirming.value = false
}

async function confirmS3() {
  const cfg: S3ImportConfig = {
    bucket: s3.value.bucket.trim(),
    prefix: s3.value.prefix?.trim() || undefined,
    region: s3.value.region?.trim() || undefined,
    accessKeyId: s3.value.accessKeyId?.trim() || undefined,
    secretAccessKey: s3.value.secretAccessKey || undefined,
    sessionToken: s3.value.sessionToken || undefined,
  }
  s3Confirming.value = false
  await startS3Import(cfg)
}

function providerBtnClass(p: Provider) {
  return cn(
    'px-4 py-1.5 text-sm rounded-md cursor-pointer transition-colors',
    provider.value === p
      ? 'bg-primary text-primary-foreground'
      : 'text-muted-foreground hover:text-foreground',
  )
}

const inputClass =
  'w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary'
</script>

<template>
  <div class="space-y-6">
    <div class="inline-flex gap-1 p-1 rounded-lg bg-muted">
      <button :class="providerBtnClass('local')" @click="provider = 'local'">
        {{ t('import.provider_local') }}
      </button>
      <button :class="providerBtnClass('s3')" @click="provider = 's3'">
        {{ t('import.provider_s3') }}
      </button>
    </div>

    <!-- LOCAL provider -->
    <template v-if="provider === 'local'">
      <Card
        v-if="stagedFiles.length === 0"
        :class="dropzoneClass"
        @dragover.prevent="dragOver = true"
        @dragenter.prevent="dragOver = true"
        @dragleave="dragOver = false"
        @drop="onDrop"
      >
        <p class="text-lg mb-2">{{ t('import.drop') }}</p>
        <p class="text-sm text-muted-foreground mb-4">{{ t('import.dedup_note') }}</p>
        <div class="flex justify-center gap-2">
          <Button variant="outline" @click="fileInput?.click()">{{ t('import.choose_files') }}</Button>
          <Button variant="outline" @click="dirInput?.click()">{{ t('import.choose_folder') }}</Button>
          <input ref="fileInput" type="file" multiple accept=".eml,.zip" class="hidden" @change="onFilePicked" />
          <input
            ref="dirInput"
            type="file"
            multiple
            class="hidden"
            @change="onFilePicked"
            v-bind="{ webkitdirectory: true, directory: true } as any"
          />
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ stagedSummary }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="clearStaged">{{ t('import.cancel') }}</Button>
            <Button @click="confirmUpload">{{ t('import.confirm') }}</Button>
          </div>
        </div>
        <ul class="text-xs font-mono space-y-1 max-h-56 overflow-auto border border-hairline rounded-sm p-3 bg-muted/30">
          <li v-for="(f, i) in stagedFiles" :key="i" class="flex justify-between gap-3">
            <span class="truncate" :title="f.name">{{ f.name }}</span>
            <span class="text-muted-foreground shrink-0">{{ formatBytes(f.size) }}</span>
          </li>
        </ul>
      </Card>
    </template>

    <!-- S3 provider -->
    <template v-else>
      <Card v-if="!s3Confirming" class="p-6 space-y-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('import.s3_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ t('import.s3_creds_hint') }}</p>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_access_key') }}</span>
            <input v-model="s3.accessKeyId" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_secret_key') }}</span>
            <input v-model="s3.secretAccessKey" type="password" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_session_token') }}</span>
            <input v-model="s3.sessionToken" type="password" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_region') }}</span>
            <input v-model="s3.region" :class="inputClass" placeholder="us-east-1" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_bucket') }} *</span>
            <input v-model="s3.bucket" :class="inputClass" autocomplete="off" />
          </label>
          <label class="space-y-1">
            <span class="text-sm">{{ t('import.s3_prefix') }}</span>
            <input v-model="s3.prefix" :class="inputClass" placeholder="mail/2026/" autocomplete="off" />
          </label>
        </div>

        <div class="flex justify-end">
          <Button :disabled="!s3Valid" @click="reviewS3">{{ t('import.s3_review') }}</Button>
        </div>
      </Card>

      <Card v-else class="p-6">
        <div class="flex items-start justify-between gap-4 mb-4">
          <div>
            <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
            <p class="text-sm text-muted-foreground">{{ t('import.s3_confirm_hint') }}</p>
          </div>
          <div class="flex gap-2 shrink-0">
            <Button variant="outline" @click="cancelS3">{{ t('import.cancel') }}</Button>
            <Button @click="confirmS3">{{ t('import.confirm') }}</Button>
          </div>
        </div>
        <dl class="text-sm space-y-2">
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_bucket') }}</dt>
            <dd class="font-mono">{{ s3.bucket }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_prefix') }}</dt>
            <dd class="font-mono">{{ s3.prefix || '—' }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_region') }}</dt>
            <dd class="font-mono">{{ s3.region || t('import.s3_region_default') }}</dd>
          </div>
          <div class="flex gap-3">
            <dt class="w-28 text-muted-foreground shrink-0">{{ t('import.s3_creds') }}</dt>
            <dd>{{ s3CredsLabel }}</dd>
          </div>
        </dl>
      </Card>
    </template>

    <!-- progress (shared) -->
    <div v-for="run in runs" :key="run.id" class="space-y-2">
      <Card class="p-4">
        <div class="flex items-center gap-3 mb-2">
          <span class="font-medium">{{ t('import.import_label', { id: run.id.slice(0, 8) }) }}</span>
          <span class="text-xs px-1.5 py-0.5 rounded bg-accent">{{ run.kind }}</span>
          <span class="ml-auto text-sm text-muted-foreground">
            <span v-if="run.total">{{ run.processed }} / {{ run.total }}</span>
            <span v-if="run.duplicates"> · {{ run.duplicates }} {{ t('import.dup') }}</span>
            <span v-if="run.errors" class="text-destructive"> · {{ run.errors }} {{ t('import.err') }}</span>
          </span>
        </div>

        <div class="flex items-center justify-between text-xs text-muted-foreground mb-1">
          <span :class="{ 'text-foreground': run.done && run.errors === 0 }">
            <span v-if="run.done && run.errors === 0">✓</span>
            <span v-else-if="run.done && run.errors > 0" class="text-destructive">✗</span>
            {{ run.phase }}
          </span>
          <span v-if="run.current" class="truncate ml-2 max-w-[50%]" :title="run.current">
            {{ run.current }}
          </span>
        </div>

        <div class="h-1.5 bg-muted rounded overflow-hidden">
          <div
            class="h-full bg-primary transition-all"
            :class="{ 'animate-pulse': !run.done && run.total === 0 }"
            :style="{
              width: run.total
                ? `${Math.min(100, (run.processed / run.total) * 100)}%`
                : (run.done ? '100%' : '15%'),
            }"
          ></div>
        </div>

        <details class="mt-3" v-if="run.log.length">
          <summary class="text-xs text-muted-foreground cursor-pointer">{{ t('import.log') }} ({{ run.log.length }})</summary>
          <ul class="mt-2 text-xs font-mono space-y-0.5 max-h-48 overflow-auto">
            <li v-for="(e, i) in run.log" :key="i" :class="{
              'text-destructive': e.kind === 'err',
              'text-muted-foreground': e.kind === 'dup',
            }">
              <span class="inline-block w-12">{{ e.kind }}</span>
              <span>{{ e.path }}</span>
              <span v-if="e.detail" class="ml-2 text-muted-foreground">{{ e.detail.slice(0, 80) }}</span>
            </li>
          </ul>
        </details>
      </Card>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Type-check (will report missing i18n at runtime only; TS should pass)**

Run: `cd web && npm run type-check`
Expected: no errors. (Missing locale keys are added in Task 10.)

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ImportPage.vue
git commit -m "feat(web): provider toggle + S3 import form on Import page"
```

---

## Task 10: i18n keys

**Files:**
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ko.json`

- [ ] **Step 1: Add the new keys to the `import` object in en.json**

Insert these key/value pairs into the existing `"import": { ... }` object in `web/src/locales/en.json` (e.g. after `"dedup_note"`):
```json
    "provider_local": "Local files",
    "provider_s3": "AWS S3",
    "s3_title": "Import from S3",
    "s3_creds_hint": "Leave credentials blank to use ~/.aws/credentials or the environment.",
    "s3_access_key": "Access Key ID",
    "s3_secret_key": "Secret Access Key",
    "s3_session_token": "Session Token",
    "s3_region": "Region",
    "s3_region_default": "from profile/environment",
    "s3_bucket": "Bucket",
    "s3_prefix": "Prefix",
    "s3_review": "Review",
    "s3_confirm_hint": "Recursively imports every .eml object under the prefix.",
    "s3_creds": "Credentials",
    "s3_creds_form": "from form",
    "s3_creds_system": "from system (~/.aws/credentials)",
```

- [ ] **Step 2: Add the matching keys to the `import` object in ko.json**

Insert into the existing `"import": { ... }` object in `web/src/locales/ko.json`:
```json
    "provider_local": "로컬 파일",
    "provider_s3": "AWS S3",
    "s3_title": "S3에서 가져오기",
    "s3_creds_hint": "자격 증명을 비워두면 ~/.aws/credentials 또는 환경 변수를 사용합니다.",
    "s3_access_key": "액세스 키 ID",
    "s3_secret_key": "시크릿 액세스 키",
    "s3_session_token": "세션 토큰",
    "s3_region": "리전",
    "s3_region_default": "프로필/환경에서 결정",
    "s3_bucket": "버킷",
    "s3_prefix": "접두사",
    "s3_review": "검토",
    "s3_confirm_hint": "접두사 아래의 모든 .eml 객체를 재귀적으로 가져옵니다.",
    "s3_creds": "자격 증명",
    "s3_creds_form": "입력값 사용",
    "s3_creds_system": "시스템 사용 (~/.aws/credentials)",
```

- [ ] **Step 3: Validate JSON + type-check + build the SPA**

Run:
```bash
cd web && npx jsonlint -q src/locales/en.json src/locales/ko.json 2>/dev/null || node -e "require('./src/locales/en.json');require('./src/locales/ko.json');console.log('json ok')"
npm run type-check
npm run build
```
Expected: JSON valid, type-check clean, `vite build` succeeds.

- [ ] **Step 4: Commit**

```bash
git add web/src/locales/en.json web/src/locales/ko.json
git commit -m "feat(web): i18n keys for S3 import provider"
```

---

## Final verification

- [ ] **Step 1: Full backend build + tests**

Run: `CGO_ENABLED=0 go build ./... && CGO_ENABLED=1 go test ./... -race -count=1`
Expected: clean build, all tests PASS.

- [ ] **Step 2: Frontend build**

Run: `cd web && npm run build`
Expected: succeeds.

- [ ] **Step 3: Maintainer smoke test (manual, not run by the agent)**

Per project convention, the maintainer runs `make build && ./local-eml serve`, opens the Import page, switches to **AWS S3**, enters a bucket (creds blank to use `~/.aws/credentials`), confirms, and watches SSE progress import every `.eml` object. Re-running reports duplicates.

---

## Self-Review Notes

- **Spec coverage:** Source interface (Task 2), local sources folded in (Task 3), generic driver (Task 4), S3 source + credential fallback (Task 5), `POST /api/imports/s3` + Source-based runJob (Task 6), frontend client/composable/UI/i18n (Tasks 7–10), tests at every backend layer with no network (fake `s3API`, real temp store). Out-of-scope items (persisting creds, other providers, parallel fetch) are intentionally excluded.
- **Type consistency:** `Source`/`Item`, `S3Config`, `NewLocalSource(label, paths, names, filterEML)`, `NewZipSource(path)`, `NewS3Source(cfg)`, `Job.RunSource`, `s3API`, `runJob(importID, src, cleanup)`, `S3ImportConfig`, `api.uploadS3`, `startS3Import` are used consistently across tasks.
- **Credentials:** never written to the `imports` row or any log line; passed straight into `S3Config` and used only for the import's lifetime.
