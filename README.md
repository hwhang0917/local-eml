# Local Eml

[English](README.md) · [한국어](README_ko.md) · **[Documentation](https://hwhang0917.github.io/local-eml/docs/)**

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/hwhang0917/local-eml/total)](https://github.com/hwhang0917/local-eml/releases)

A private, local viewer for `.eml` files. Drop a folder, a `.zip`, an `.mbox`, a `.pst`, or pull from S3 / IMAP — search, browse, and read your mail offline. Everything stays on your machine.

**It is** a local archive for your email: pull old messages out of an overflowing mailbox, keep them as plain `.eml` files on your own disk, and search and read them any time.

**It isn't** an email client. You can't compose, send, or reply, and it never modifies your mailbox — IMAP access is strictly read-only. It's desktop only, and it listens on `127.0.0.1` alone.

## Download

Download the file for your OS from the [latest release](https://github.com/hwhang0917/local-eml/releases/latest) and double-click it. Local Eml opens in its own window; close the window and it shuts down. No installer, no account. The [install guide](https://hwhang0917.github.io/local-eml/docs/install) explains the first-run prompt on Windows and macOS.

**winget:** `winget install hwhang0917.LocalEml` (listing pending review).

**Run it all the time instead?** One line installs it as a background service that starts on boot and keeps fetching new IMAP mail:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

Then open <http://localhost:7878> and paste the token that `local-eml token` prints, or double-click the program to open a signed-in window. Ports, verifying downloads, uninstalling and updating are in the [install guide](https://hwhang0917.github.io/local-eml/docs/install).

## Learn more

Step-by-step: [open .eml files](https://hwhang0917.github.io/local-eml/docs/how-to/open-eml-files) · [view a Google Takeout .mbox](https://hwhang0917.github.io/local-eml/docs/how-to/takeout-mbox) · [open a .pst without Outlook](https://hwhang0917.github.io/local-eml/docs/how-to/pst-without-outlook) · [free up Gmail storage](https://hwhang0917.github.io/local-eml/docs/how-to/free-up-gmail)

The **[documentation](https://hwhang0917.github.io/local-eml/docs/)** covers everything else:

- [Importing mail](https://hwhang0917.github.io/local-eml/docs/import) from files, folders, archives, `.mbox`, `.pst`, S3 and IMAP
- [Searching](https://hwhang0917.github.io/local-eml/docs/search), including Korean 초성검색, filters and performance
- [Receiving new mail](https://hwhang0917.github.io/local-eml/docs/sync) from IMAP in the background
- [Spam & phishing](https://hwhang0917.github.io/local-eml/docs/spam-phishing) warnings and flags
- [Privacy & your data](https://hwhang0917.github.io/local-eml/docs/privacy): where data lives, credentials, temporary sessions, deleting everything
- [Settings & CLI](https://hwhang0917.github.io/local-eml/docs/settings-cli) and [troubleshooting](https://hwhang0917.github.io/local-eml/docs/troubleshooting)

## Contributing

See [`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) to build or run from source, and [`CONTRIBUTING.md`](CONTRIBUTING.md) before opening an issue or pull request. Library attributions are listed in-app under **Settings → Attributions**.

## License

[GNU General Public License v3.0](LICENSE)
