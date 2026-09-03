# Local Eml

[English](README.md) · [한국어](README_ko.md)

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

A private, local viewer for `.eml` files. Drop a folder, a `.zip`, or pull from S3 / IMAP — search, browse, and read your mail offline. Everything stays on your machine.

## What it is — and what it isn't

**It is** a local archive for your email. Pull old messages out of an overflowing mailbox, keep them as plain `.eml` files on your own disk, and search and read them any time — freeing up space in your mail account without losing anything.

**It isn't** a full email client like Thunderbird or Outlook. You can't compose, send, or reply, and it never modifies your mailbox — IMAP access is strictly read-only, only for fetching copies. Keep your regular client for day-to-day mail; use Local Eml for the mail you want to keep but not carry.

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

### Or just double-click it — no install

Not comfortable with terminals? Download the file for your OS from the [latest release](https://github.com/hwhang0917/local-eml/releases/latest) and double-click it. Local Eml opens in its own browser window, and when you close the window it shuts itself down a few minutes later — nothing keeps running in the background.

In this mode new mail is only fetched while the window is open; use the installer above if you want mail synced around the clock.

## What you can do

- **Import** emails from `.eml` files, folders, `.zip` archives, an **AWS S3** bucket, or an **IMAP** mailbox. Duplicates are detected by file hash and skipped. In-flight imports can be cancelled at any time.
- **Search** by sender, subject, or body. Korean and other CJK languages work out of the box, and typing only Hangul initial consonants (e.g. `ㅎㄱ` to find `한국`) does **초성검색** across the whole library.
- **Receive new mail** automatically from any saved IMAP profile (opt-in per profile). Local Eml fetches only what's new since the last sync; every 10 minutes by default, adjustable on the Import page.
- **Read safely** — HTML messages render in a sandboxed iframe with remote images blocked by default.
- **Star** messages you want to revisit and filter to just starred.
- **Export** your library as a single `.zip` or upload it to an S3 bucket. Existing keys in the destination are skipped, so re-running is safe.
- **Save profiles** for IMAP and S3 so you don't retype host names and bucket details.
- **Multilingual interface** and **absolute / relative** date display, switchable in Settings.

Your data lives in `~/.local-eml/` (or `%USERPROFILE%\.local-eml\` on Windows). Local Eml only listens on `127.0.0.1` — nothing is exposed to your network. A red banner shows up at the top of the page if the background service stops responding.

## Performance

How does it hold up as the library grows? Benchmarked with synthetic catalogs on a mid-range laptop (12th-gen i5, WSL2), timing one 50-row page of the library API including its total count:

| Operation | 10k emails | 100k emails |
|---|---|---|
| Browse / paginate | < 1 ms | < 1 ms |
| Search, typical term | < 1 ms | ~2 ms |
| Search, term matching nearly every email | ~20 ms | ~200 ms |
| 초성 search | ~5 ms | ~50 ms |
| Conversation-grouped list | ~85 ms | ~0.9 s |
| Grouped list + search | ~100 ms | ~1 s |

Browsing and full-text search (SQLite FTS5) stay effectively instant well past 100k messages. The conversation-grouped view is the one thing that grows with catalog size — if pages feel slow on a very large library, turn the grouping toggle off. Reproduce with:

```bash
go test ./internal/store -bench ListEmails -benchtime 5x -run '^$'
LOCAL_EML_BENCH_N=100000 go test ./internal/store -bench ListEmails -benchtime 5x -run '^$'
```

## Where credentials go

- **AWS S3 secret key, session token** — never persisted. You re-enter them per import / export.
- **IMAP password** — by default, never persisted; you re-enter it per import.
  - If you turn on **"Receive new mail in the background"** on a saved IMAP profile, Local Eml needs to log in unattended. The password is then encrypted with AES-256-GCM and stored in the database; the encryption key lives in a separate file at `~/.local-eml/keys/secret.key` (mode `0600`). A leaked or backed-up database alone does not expose the password — the attacker also needs the keyfile. Turning the toggle off removes the stored password and sync state on the next save.
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

## License

[GNU General Public License v3.0](LICENSE)
