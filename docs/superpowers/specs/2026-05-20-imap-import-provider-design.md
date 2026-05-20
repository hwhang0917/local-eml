# IMAP Import Provider — Design

Date: 2026-05-20

## Goal

Add a third import provider to 가져오기 (Import), after local upload and AWS S3: **IMAP**.
The user enters server connection details (host, optional port, username, password) and a
mailbox folder (default `INBOX`); the server connects read-only, lists every message in that
folder, and imports each raw RFC822 message through the existing dedup/parse/store pipeline.

The provider plugs into the existing `Source` / `SourceCloser` interface introduced for S3, so
the import driver, SSE progress, dedup, and frontend progress UI are unchanged.

## Background

The importer already separates per-item work (`Importer.ImportReader(ctx, io.Reader, name)` —
SHA-256, dedup, parse, blob write, FTS insert) from per-source orchestration
(`Job.RunSource(ctx, Source)` — scan, set total, loop, emit `step`/`item`/`done`/`error`). A mail
message is RFC822, which is exactly what `ImportReader` parses, so an IMAP message body streams in
directly. `SourceCloser` (a `Source` that is also an `io.Closer`) already exists for the zip
provider and gives us connection cleanup for free.

## Architecture

### Lazy fetch model

IMAP connections are stateful and process one command at a time, so the mailbox is never loaded
eagerly. Mirroring the S3 list-then-get shape:

1. `Scan` connects, authenticates, opens the folder **read-only** (`EXAMINE`, not `SELECT`), and
   issues `UID SEARCH ALL` — cheap, returns only the UID list. One `Item` is produced per UID.
2. `Item.Open(uid)` issues `UID FETCH <uid> BODY.PEEK[]`, buffers that single message's raw bytes,
   and returns `io.NopCloser(bytes.NewReader(buf))`. Only one message is held in memory at a time.
   `BODY.PEEK[]` (not `BODY[]`) means importing never sets the `\Seen` flag on the server.

This keeps memory bounded for large folders and matches the S3 provider's structure exactly.

### Source implementation

`internal/importer/source_imap.go`:

```go
type IMAPConfig struct {
	Host     string
	Port     int    // 0 -> default 993
	Username string
	Password string
	Folder   string // "" -> "INBOX"
}
```

- `imapSource{cfg IMAPConfig; session imapSession; dial func(IMAPConfig) (imapSession, error)}`
  implements `SourceCloser`.
- `Label()` -> `imap://<username>@<host>/<folder>`.
- `Scan`: lazily builds the session via `dial`, then `session.UIDs()`; for each UID an `Item` whose
  `Open` calls `session.Fetch(uid)`.
- `Close()`: `session.Close()` (logout + close connection), nil-safe.

### Test seam

go-imap's fetch API is callback/command-based and awkward to fake, so it is isolated behind a
small interface:

```go
type imapSession interface {
	UIDs() ([]imap.UID, error)
	Fetch(uid imap.UID) ([]byte, error)
	Close() error
}
```

`imapSource` depends only on `imapSession`. Unit tests inject a fake session (in-memory UID→bytes
map, like `fakeS3` for the S3 source) and verify: UID listing maps to items, `Open` returns the
right body, a `Fetch` error surfaces per-item (driver continues), and `Label` formatting. The real
session — `newIMAPSession(cfg)` doing dial + login + `EXAMINE` + the go-imap fetch plumbing — is a
thin adapter exercised by manual integration test against a real server (per project convention,
the maintainer runs it).

### Connection details

- Port default 993. Port 993 → implicit TLS (`tls.Dial`); port 143 → plaintext dial then
  `STARTTLS`. Chosen by port number.
- Auth: `LOGIN` with username/password (app-passwords for providers like Gmail). No OAuth in v1.
- Read-only throughout (`EXAMINE`); the importer never writes to the mail server.

### Dependency

`github.com/emersion/go-imap/v2 v2.0.0-beta.8` (resolved/pinned via `go get`; `go mod tidy`).
Pure Go, cross-compiles under `CGO_ENABLED=0`. It is a long-lived beta and the maintained line
(v1 is in maintenance only); the beta API risk is accepted for this personal-scale tool.

## API

```
POST /api/imports/imap
  body: {host, port?, username, password, folder?}
  202:  {import_id, kind:"imap"}   (Accepted — import runs async; matches the other endpoints)
  400:  host / username / password missing
```

`handleImportImap` decodes and trims the body, validates host/username/password non-empty, creates
the `imports` row with `source_kind="imap"`, `source_name="imap://user@host/folder"`,
`status="queued"`, builds `importer.NewIMAPSource(cfg)`, and launches `runJob(importID, src, noop)`.
Route registered as `api.Post("/imports/imap", s.handleImportImap)`.

Password travels as plain JSON to localhost (loopback-only single-user app, same accepted decision
as S3 credentials). It is passed only into `IMAPConfig`, never written to the import row (only the
`imap://user@host/folder` label is stored) and never logged.

## Frontend

`web/src/pages/ImportPage.vue`: the provider toggle gains a third option →
**로컬 파일 | AWS S3 | IMAP**. The IMAP pane is a form: Host, Port (default 993, number),
Username, Password (`type="password"`), Folder (default `INBOX`). Same two-step pattern as the
others: fill form → confirm summary (host / username / folder / port) → start. Validation: Host,
Username, Password required (Review button disabled until all three are non-empty).

`web/src/lib/api.ts`: `IMAPImportConfig` type + `uploadImap(cfg)` (POST JSON).
`web/src/composables/useImports.ts`: `startImapImport(cfg)` — POST then create an `ImportRun`
(`kind:"imap"`, `total:0` until the scan reports it) and call the existing `followProgress`.
i18n: new `import.*` keys for the IMAP tab and field labels/placeholders/confirm summary, added to
both `en.json` and `ko.json` in parity.

## Testing

- `imapSource.Scan` maps UIDs to items; `Item.Open` returns the matching body — fake `imapSession`.
- A `Fetch` error on one UID surfaces as an item error and does not abort the batch (driven through
  `Job.RunSource` with a real temp store, like the S3/RunSource tests).
- `Label()` formatting.
- Existing importer/server/web tests still pass.

Per project convention the maintainer runs build/serve and the real-server smoke test; this spec
lists commands but does not run them.

## Out of scope (v1)

- Multi-folder / recursive import (one folder per import; user can target e.g. `[Gmail]/All Mail`).
- OAuth / Gmail XOAUTH2 (app-passwords cover the common case).
- Incremental / since-UID sync (each import re-scans; SHA-256 dedup skips already-imported mail).
- Saving connection profiles or credentials in-app.
- POP3 (legacy; deliberately not pursued).
