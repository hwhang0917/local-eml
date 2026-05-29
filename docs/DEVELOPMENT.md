# Development

## Requirements

- Go ≥ 1.25
- Node ≥ 22
- `jq`

## Build & run

```bash
git clone https://github.com/hwhang0917/local-eml.git
cd local-eml

make install            # npm install (web/) + go mod download
make build              # syncs VERSION → web/package.json, builds SPA, builds Go binary
./local-eml serve       # http://localhost:7878
```

Common make targets:

```text
make build        sync-version + web-build + go-build (single self-contained binary)
make web-dev      Vite dev server on :5173 (proxies /api → :7878)
make run          go run ./cmd/local-eml serve
make check        gofmt + go vet + go test ./... -race
make cross        Cross-compile all 6 platforms into dist/
```

## CLI

```text
local-eml <command> [flags]

  serve [--port 7878]      run the local web server (loopback only)
  install [-y|--yes]       register as a background service (systemd/launchd/svc)
  uninstall [-y|--yes]     stop and unregister the background service
  version | -V | --version
```

`install` / `uninstall` prompt `[Y/n]` (Enter accepts) by default; `--yes` skips for automation.

## Data layout

- `~/.local-eml/eml/<sha256>.eml` — content-addressed raw blobs
- `~/.local-eml/db/local-eml.db` — SQLite (with FTS5)
- `~/.local-eml/logs/local-eml.log` — append-only log; mirrors stderr

`%USERPROFILE%\.local-eml\` on Windows.

## Inspecting logs

```bash
# Linux (systemd user)
journalctl --user -u local-eml -f
journalctl --user -u local-eml -o json | jq

# macOS (launchd)
log show --predicate 'process == "local-eml"' --info --last 1h

# Anywhere
tail -f ~/.local-eml/logs/local-eml.log
```

Set `LOCAL_EML_LOG_LEVEL=debug|info|warn|error` in the service environment to tune verbosity.

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│ local-eml (single Go binary, loopback-only)                    │
│                                                                │
│   Vue 3 SPA  ←  go:embed web/dist                              │
│       │                                                        │
│       │ /api/...   /api/imports/:id/events (SSE)               │
│       ▼                                                        │
│   HTTP server (chi router)                                     │
│       │                                                        │
│       ├─► Importer — pluggable Sources behind one              │
│       │     driver: local file/dir/zip, AWS S3, IMAP;          │
│       │     SHA-256 dedup, async via Hub pub-sub (SSE)         │
│       │                                                        │
│       ├─► Exporter — writes the library to a streaming         │
│       │     zip, or uploads to S3 (skip-existing)              │
│       │                                                        │
│       ├─► Sanitizer (bluemonday + cid rewrite +                │
│       │     remote-image gating)                               │
│       │                                                        │
│       └─► Store (SQLite + FTS5)                                │
└────────────────────────────────────────────────────────────────┘
```

| Layer | Stack |
|---|---|
| Backend | Go, chi, `modernc.org/sqlite` (pure-Go), enmime, bluemonday, kardianos/service, aws-sdk-go-v2 (S3), go-imap/v2 (IMAP) |
| Frontend | Vue 3, Vite, TypeScript, Tailwind v4, reka-ui (shadcn-vue primitives), vue-i18n, vue-sonner |
| Service | systemd user-unit (Linux), LaunchAgent (macOS), Windows Service |

## Releasing

`VERSION` at the repo root is the single source of truth. `scripts/release.sh` bumps it, propagates the new version into `web/package.json` and `web/package-lock.json` (via `npm version`), then creates a single `Release version vX.X.X` commit and a matching annotated tag — after a `[y/N]` confirmation. It does not push.

```bash
scripts/release.sh patch       # 0.0.1 -> 0.0.2   (also: minor, major)
git push --follow-tags         # push the commit + tag when ready
```

GitHub Actions (`.github/workflows/release.yml`) cuts the release: cross-compiles six binaries (`linux|darwin|windows`-`amd64|arm64`), generates `SHA256SUMS`, and attaches everything to a Release whose body is auto-generated from PR titles since the previous tag.
