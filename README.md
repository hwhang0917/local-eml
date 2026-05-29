# Local Eml

[English](README.md) · [한국어](README_ko.md)

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)

A private, local viewer for `.eml` files. Drop a folder, a `.zip`, or pull from S3 / IMAP — search, browse, and read your mail offline. Everything stays on your machine.

## Install

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

The installer downloads the latest release, verifies it, and registers Local Eml as a background service so it starts automatically. Open <http://localhost:7878> in your browser.

## What you can do

- **Import** emails from `.eml` files, folders, `.zip` archives, an **AWS S3** bucket, or an **IMAP** mailbox. Duplicates are detected by file hash and skipped.
- **Search** by sender, subject, or body — including Korean and other CJK languages.
- **Read safely** — HTML messages render in a sandboxed iframe with remote images blocked by default.
- **Star** messages you want to revisit.
- **Export** your library as a single `.zip` or upload it back to an S3 bucket.
- **Save profiles** for IMAP and S3 so you don't retype host names and bucket details (passwords and secret keys are never stored).
- **Korean / English** interface, switchable in Settings.

Your data lives in `~/.local-eml/` (or `%USERPROFILE%\.local-eml\` on Windows). Local Eml only listens on `127.0.0.1` — nothing is exposed to your network.

## Uninstall

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

Add `--purge` (or `-Purge` on Windows) to also delete the data folder. Without it, your mail library stays.

## Building from source

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) if you want to build, run, or contribute. Library attributions are listed in-app under **Settings → Attributions**.
