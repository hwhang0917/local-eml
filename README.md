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

- **Import** emails from `.eml` files, folders, `.zip` archives, an **AWS S3** bucket, or an **IMAP** mailbox. Duplicates are detected by file hash and skipped. In-flight imports can be cancelled at any time.
- **Search** by sender, subject, or body. Korean and other CJK languages work out of the box, and typing only Hangul initial consonants (e.g. `ㅎㄱ` to find `한국`) does **초성검색** across the whole library.
- **Receive new mail** automatically from any saved IMAP profile (opt-in per profile). Local Eml fetches only what's new since the last sync; default cadence is every 10 minutes.
- **Read safely** — HTML messages render in a sandboxed iframe with remote images blocked by default.
- **Star** messages you want to revisit and filter to just starred.
- **Export** your library as a single `.zip` or upload it to an S3 bucket. Existing keys in the destination are skipped, so re-running is safe.
- **Save profiles** for IMAP and S3 so you don't retype host names and bucket details.
- **Korean / English** interface and **absolute / relative** date display, switchable in Settings.

Your data lives in `~/.local-eml/` (or `%USERPROFILE%\.local-eml\` on Windows). Local Eml only listens on `127.0.0.1` — nothing is exposed to your network. A red banner shows up at the top of the page if the background service stops responding.

## Where credentials go

- **AWS S3 secret key, session token** — never persisted. You re-enter them per import / export.
- **IMAP password** — by default, never persisted; you re-enter it per import.
  - If you turn on **"Receive new mail in the background"** on a saved IMAP profile, Local Eml needs to log in unattended. The password is then encrypted with AES-256-GCM and stored in the database; the encryption key lives in a separate file at `~/.local-eml/keys/secret.key` (mode `0600`). A leaked or backed-up database alone does not expose the password — the attacker also needs the keyfile. Turning the toggle off removes the stored password on the next save.
- **Other profile fields** (host, bucket, region, username, access-key-id, etc.) — saved in plain SQLite, since they're not secrets.

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
