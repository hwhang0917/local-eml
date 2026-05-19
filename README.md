# Local Eml

A local-first viewer for `.eml` files. Drop a folder or a zip on it, search across thousands of messages, render HTML bodies safely in a sandboxed iframe, tag for later. Single Go binary with an embedded Vue SPA — runs on `http://localhost:7878`, your data never leaves your machine.

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)

## Features

- **Import**: single files, directories (browser-`webkitdirectory`), or `.zip` archives — with a two-step confirmation, live SSE progress, and SHA-256 deduplication.
- **Library**: virtualizable list with sort + FTS5 fuzzy search across subject, from, to, and body. CJK queries supported via per-term prefix matching.
- **Viewer**: sandboxed iframe + strict CSP for HTML bodies; inline `cid:` images rewritten; remote images blocked by default with a one-click toggle. Plain-text and raw `.eml` tabs alongside.
- **Tags**: shadcn-style `TagsInput` with paste-to-split; tag-based library filtering.
- **i18n**: English / Korean, with locally vendored Roboto (Latin) and Pretendard (Hangul) fonts.
- **Cross-platform service**: `local-eml install` registers a user-level systemd unit on Linux, a LaunchAgent on macOS, and a Windows Service. Logs adapt to interactive vs service mode.

## Install

### Linux / macOS

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

The installer downloads the matching prebuilt binary from the latest GitHub Release, verifies it against `SHA256SUMS`, places it in:

- `~/.local/bin/local-eml` on Linux / macOS
- `%LOCALAPPDATA%\local-eml\local-eml.exe` on Windows

then runs `local-eml install` to register the service. Once it's running, open <http://localhost:7878>.

Install flags:

```text
--version <tag>    Pin a release (e.g. v0.0.1). Default: latest
--dir <path>       Install directory
--no-service       Skip service registration
```

### Uninstall

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh -s -- --purge

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

`--purge` (or `-Purge`) also deletes `~/.local-eml/` (EMLs + DB + logs). Without it, data is preserved.

## CLI

```text
local-eml <command> [flags]

  serve [--port 7878]      run the local web server (loopback only)
  install [-y|--yes]       register as a background service (systemd/launchd/svc)
  uninstall [-y|--yes]     stop and unregister the background service
  version | -V | --version
```

`install` / `uninstall` prompt `[Y/n]` (Enter accepts) by default; `--yes` skips for automation.

## Where data lives

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

## Build from source

Requirements: Go ≥ 1.25, Node ≥ 22, `jq`.

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

## Architecture

```
┌────────────────────────────────────────────────────────────────┐
│ local-eml (single Go binary, loopback-only)                    │
│                                                                │
│  Vue 3 SPA  ←  go:embed web/dist                               │
│      │                                                         │
│      │ /api/...   /api/imports/:id/events (SSE)                │
│      ▼                                                         │
│  HTTP server (chi router)                                      │
│      │                                                         │
│      ├─► Importer (worker pool, SHA-256 dedup,                 │
│      │   zip/dir/file walkers, async via Hub pub-sub)          │
│      │                                                         │
│      ├─► Sanitizer (bluemonday + cid rewrite +                 │
│      │   remote-image gating)                                  │
│      │                                                         │
│      └─► Store (SQLite + FTS5)                                 │
└────────────────────────────────────────────────────────────────┘
```

| Layer | Stack |
|---|---|
| Backend | Go, chi, `modernc.org/sqlite` (pure-Go), enmime, bluemonday, kardianos/service |
| Frontend | Vue 3, Vite, TypeScript, Tailwind v4, reka-ui (shadcn-vue primitives), vue-i18n, vue-sonner |
| Service | systemd user-unit (Linux), LaunchAgent (macOS), Windows Service |

Open-source attributions are listed in-app under **Settings → Attributions**.

## Releasing

`VERSION` at the repo root is the single source of truth.

```bash
# Edit VERSION (e.g. 0.0.2)
make sync-version              # writes the same value into web/package.json
git commit -am "release: v0.0.2"
git tag v0.0.2
git push origin main --tags
```

GitHub Actions (`.github/workflows/release.yml`) cuts the release: cross-compiles six binaries (`linux|darwin|windows`-`amd64|arm64`), generates `SHA256SUMS`, and attaches everything to a Release whose body is auto-generated from PR titles since the previous tag.

## Status

`v0.0.x`. Personal-scale tool. Single-user, loopback-only, no auth.
