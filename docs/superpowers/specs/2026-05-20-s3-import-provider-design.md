# S3 Import Provider — Design

Date: 2026-05-20

## Goal

Add a second import provider to 가져오기 (Import) beyond local file upload: **AWS S3**.
The user enters optional AWS credentials (falling back to `~/.aws/credentials` / the SDK
default chain if blank), a bucket name, and an optional key prefix; the server recursively
fetches every `*.eml` object and imports it through the existing dedup/parse/store pipeline.

Provider logic is placed behind a `Source` interface so future providers (GCS, IMAP, …) slot
in without touching the import driver. To prove the abstraction, the existing local providers
(`file` / `dir` / `zip`) are also reimplemented as `Source`s.

## Background

The current import pipeline (`internal/importer`) cleanly separates two concerns:

- `Importer.ImportReader(ctx, io.Reader, name)` and `ImportFile` — per-item work: SHA-256,
  dedup check, parse (`enmime`), atomic blob write, transactional metadata + FTS insert.
- `Job.RunFile` / `RunDir` / `RunZip` — per-source orchestration: scan, set total, loop items,
  emit SSE `step`/`item`/`done`/`error` events, update `imports` counters.

`ImportReader` already accepts any `io.Reader`, so an S3 object body streams in directly. The
SSE `Hub`, the `Event` struct, and the frontend `followProgress` loop are all source-agnostic —
they carry any import kind without change.

## Architecture

### Source interface

New file `internal/importer/source.go`:

```go
type Item struct {
    Name string                                  // logical path for reporting (e.g. key or filename)
    Open func(ctx context.Context) (io.ReadCloser, error)
}

type Source interface {
    Label() string                               // human label for the "Scanning <label>" phase
    Scan(ctx context.Context) ([]Item, error)    // candidate items, already filtered to .eml
}
```

`Item.Open` is a lazy closure so object bodies are streamed one at a time (memory stays bounded
for large buckets/archives); keys/paths themselves are cheap to hold in a slice.

### Generic driver

`Job.RunSource(ctx, src Source)` replaces the three bespoke `Run*` methods:

1. `step` "Scanning <label>".
2. `items, err := src.Scan(ctx)` — on error: mark import `error`, emit `error` event, return.
3. `SetImportTotal(len(items))`; `step` "Importing N emails".
4. For each item (honoring `ctx` cancellation): `rc := it.Open(ctx)` → `Importer.ImportReader(ctx, rc, it.Name)` → `rc.Close()`.
   - per-item error → `RecordImportError` + `IncImportCounters(1,0,1)` + `item` event with message.
   - success → `IncImportCounters(1,dup,0)` + `item` event with sha/duplicate.
5. `step` "Finalizing"; `UpdateImportStatus(done)`; `done` event.

This is the existing `RunDir` body generalized; per-item error handling and counter semantics are
preserved exactly.

### Source implementations

- `internal/importer/source_local.go`
  - `localFilesSource{paths, names []string}` — covers `file` (1 entry) and `dir` (N entries);
    `Open` opens the temp file by path. Filters `.eml` for the dir case.
  - `zipSource{path string}` — opens the archive in `Scan`, returns one `Item` per `.eml` entry
    whose `Open` re-opens that `*zip.File`. The `*zip.ReadCloser` is held by the source and
    closed when the job finishes (via an explicit `Close()` the driver defers if the source is an
    `io.Closer`).
- `internal/importer/source_s3.go`
  - `s3Source{cfg S3Config}` with `S3Config{AccessKeyID, SecretAccessKey, SessionToken, Region, Bucket, Prefix string}`.
  - `Scan`: build client, `ListObjectsV2` paginated over `Prefix`, collect keys ending `.eml`
    (reusing `isEML`). `Open(key)` issues `GetObject` and returns the body `io.ReadCloser`.
  - `Label()`: `s3://<bucket>/<prefix>`.

### Credentials

In `s3Source` client construction:

- If `cfg.AccessKeyID != ""` → `config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(AccessKeyID, SecretAccessKey, SessionToken))`.
- Else → omit; `config.LoadDefaultConfig` uses the SDK default chain (`~/.aws/credentials`, env, IAM).
- `cfg.Region` applied via `config.WithRegion` when non-empty; otherwise resolved from profile/env.

Credentials are used for the lifetime of the import only — **never persisted, never logged**.
The `S3Config` is not stored in the `imports` table and is kept out of any log line.

### Dependency

`github.com/aws/aws-sdk-go-v2` family — packages `config`, `credentials`, `service/s3` (and
transitive `aws`). Pure Go; cross-compiles under `CGO_ENABLED=0`. Versions resolved via
`go get @latest` + `go mod tidy` (no guessed pins, per project convention).

## API

New endpoint, JSON (not multipart):

```
POST /api/imports/s3
  body: {accessKeyId?, secretAccessKey?, sessionToken?, region?, bucket, prefix?}
  202:  {import_id, kind:"s3"}   (Accepted — import runs async; matches POST /api/imports)
  400:  bucket missing
```

`handleImportS3` decodes the body, validates `bucket`, creates the `imports` row with
`source_kind="s3"`, `source_name="s3://bucket/prefix"`, launches `runJob` in a goroutine, and
returns the id. The multipart endpoint (`POST /api/imports`) is unchanged.

`runJob` is refactored to build a `Source` and call `job.RunSource`:

- multipart upload handler builds a `localFilesSource` or `zipSource` from temp paths.
- S3 handler passes a prebuilt `s3Source`.

(The handler still owns temp-file cleanup for local uploads via the existing `removeAll` defer.)

## Frontend

`web/src/pages/ImportPage.vue`:

- A **provider segmented control** at the top: **로컬 파일** (default) | **AWS S3**.
- **로컬 파일** pane: the existing dropzone + two-step confirm, unchanged.
- **AWS S3** pane: a form with fields — Access Key ID, Secret Access Key, Session Token
  (optional; helper text "비워두면 ~/.aws/credentials 사용"), Region (optional), Bucket
  (required), Prefix (optional). Same two-step pattern: **fill form → confirm summary**
  (bucket / prefix / region / "credentials: 입력값" or "시스템") **→ start**.

`web/src/composables/useImports.ts`: add `startS3Import(cfg)` that POSTs JSON via a new
`api.uploadS3(cfg)`, then creates the `ImportRun` (`kind:"s3"`, `total:0` until the scan reports
it) and calls the existing `followProgress`. No changes to the SSE event handling.

`web/src/lib/api.ts`: add `uploadS3(cfg): Promise<{import_id, kind}>`.

i18n: new keys under `import.*` in `web/src/locales/{en,ko}.json` for the provider tabs, S3 field
labels/placeholders/helper text, and the confirm summary.

## Testing

- `s3Source.Scan` key filtering + pagination — unit test against a faked `ListObjectsV2`
  paginator (interface seam over the S3 client so no network).
- `localFilesSource` / `zipSource` `Scan`+`Open` — table tests over a temp dir and a temp zip,
  asserting `.eml` filtering and that bodies read back correctly.
- `Job.RunSource` — drive with a stub `Source` (in-memory items, one erroring `Open`), assert
  emitted events, counters, and that one bad item does not abort the batch.
- Existing importer/sse tests must still pass after the `Run*` → `RunSource` refactor.

Per project convention, the user runs builds/tests; this spec lists the commands but does not run
them.

## Out of scope (v1)

- Persisting credentials or AWS profiles in-app.
- Other providers (GCS, Azure Blob, IMAP) — the interface is the seam for them, but only S3 ships.
- S3 bucket browsing / object preview UI.
- Parallel object fetch (sequential, matching current dir/zip behavior).
