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
make build              # copies VERSION into web/package.json, builds the SPA, then the Go binary
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

  app [--ephemeral]        open in a browser window; stops when the window closes (default)
  serve [--port 7878] [--ephemeral]
                           run the local web server (loopback only)
  install [-y|--yes] [--port 7878]
                           register as a background service (systemd/launchd/Windows Service)
  uninstall [-y|--yes]     stop and unregister the background service
  token                    print the session token the API and a browser need
  version | -V | --version
```

`--ephemeral` (or `LOCAL_EML_EPHEMERAL=1`) runs a temporary session: the data folder is a fresh
`local-eml-session-*` directory under the OS temp dir, removed on shutdown, and leftovers from
crashed sessions are swept on the next launch. It is ignored when running as a service.

`install` / `uninstall` prompt `[Y/n]` (Enter accepts) by default; `--yes` skips for automation.

## Data layout

- `~/.local-eml/eml/<sha256>.eml`: raw messages, named by content hash
- `~/.local-eml/db/local-eml.db`: SQLite with FTS5
- `~/.local-eml/logs/local-eml.log`: append-only log, same as stderr. Rotates daily to `local-eml-YYYY-MM-DD.log`; rotated days older than 30 days are deleted (`internal/logrotate`)
- `~/.local-eml/keys/secret.key`: key for stored IMAP passwords, only when no OS keychain answers. Otherwise the key is in the keychain (service `local-eml`, account = absolute key file path) and `keys/secret.key.in-keychain` marks that. `LOCAL_EML_KEYRING=off` forces the file; temporary sessions always use the file (`internal/secret`)
- `~/.local-eml/session-token`: API session token

`%USERPROFILE%\.local-eml\` on Windows.

## Inspecting logs

```bash
# Linux (systemd user)
journalctl --user -u local-eml -f
journalctl --user -u local-eml -o json | jq

# macOS (launchd)
log show --predicate 'process == "local-eml"' --info --last 1h

# Anywhere
tail -F ~/.local-eml/logs/local-eml.log
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
| Frontend | Vue 3, Vite, TypeScript, Tailwind v4, reka-ui (shadcn-vue primitives), vue-i18n, vue-sonner; attachment previews: rhwp (HWP/HWPX, WASM), docx-preview, SheetJS (vendored in `web/vendor/`, from cdn.sheetjs.com) |
| Service | systemd user-unit (Linux), LaunchAgent (macOS), Windows Service |

## Third-party license notices

`npm run build` writes `web/dist/THIRD_PARTY_NOTICES.txt`: the license texts and NOTICE files of every npm package in the bundle (found from the bundle itself, see `web/third-party/notices.ts`) and every Go module compiled into the binary. The app serves it from **Settings → Attributions**, and each GitHub release attaches it next to `LICENSE.txt`.

The Go half comes from `web/third-party/go-notices.txt`, which is committed because the web build has no Go toolchain in CI. After changing `go.mod`, regenerate it:

```sh
go run scripts/gen-go-notices.go
```

CI fails when it is stale. An npm package published without a license file fails the web build until its upstream license is added as `web/third-party/licenses/<name>.txt` (scoped names use `__` for `/`).

## Releasing

`VERSION` at the repo root is the single source of truth. `scripts/release.sh` bumps it, propagates the new version into `web/package.json` and `web/package-lock.json` (via `npm version`), then asks `[y/N]` and creates a single `Release version vX.X.X` commit with a matching annotated tag. It does not push.

```bash
scripts/release.sh patch       # 0.0.1 -> 0.0.2   (also: minor, major)
git push --follow-tags         # push the commit + tag when ready
```

GitHub Actions (`.github/workflows/release.yml`) then builds the release: it cross-compiles six binaries (`linux|darwin|windows`-`amd64|arm64`), generates `SHA256SUMS`, and attaches everything to a Release whose body is auto-generated from PR titles since the previous tag.
