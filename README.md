# Local Eml

[English](README.md) · [한국어](README_ko.md) · [日本語](README_ja.md) · **[Documentation](https://hwhang0917.github.io/local-eml/docs/)**

[![CI](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/ci.yml)
[![Release](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml/badge.svg)](https://github.com/hwhang0917/local-eml/actions/workflows/release.yml)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/hwhang0917/local-eml/total)](https://github.com/hwhang0917/local-eml/releases)

Local Eml keeps your old email on your own computer and lets you search and read it offline. You can give it `.eml` files, a folder, a `.zip`, an `.mbox` or an Outlook `.pst`, or pull mail from an IMAP account or an S3 bucket (AWS or an S3-compatible service such as MinIO, Cloudflare R2 or Backblaze B2). Your mail never leaves your machine.

It's meant for archiving. If your mailbox is full, you can copy the old messages out, keep them as ordinary `.eml` files, and still find them later.

It is not an email client. You can't write, send or reply to mail, and it never changes anything in your mailbox: IMAP access is read-only. It runs on desktop computers only, and only your own computer can connect to it.

## Download

Get the file for your system from the [latest release](https://github.com/hwhang0917/local-eml/releases/latest) and double-click it. Local Eml opens in its own window and quits when you close it. There's no installer and no account. Windows and macOS warn you the first time you open it; the [install guide](https://hwhang0917.github.io/local-eml/docs/install) shows how to get past that.

On Windows you'll also be able to use `winget install hwhang0917.LocalEml` once the listing is approved.

If you'd rather have it running all the time, you can install it as a background service. It starts when the computer does and can keep fetching new mail from your IMAP accounts.

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

```powershell
# Windows (PowerShell)
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

Then go to <http://localhost:7878>. The first time, it asks for a session token, and the page shows how to get one with the `local-eml token` command. Double-clicking the program also works and signs you in for you. The [install guide](https://hwhang0917.github.io/local-eml/docs/install) covers changing the port, checking downloads, updating and uninstalling.

## Documentation

Step-by-step guides:

- [Open .eml files](https://hwhang0917.github.io/local-eml/docs/how-to/open-eml-files)
- [View a Google Takeout .mbox](https://hwhang0917.github.io/local-eml/docs/how-to/takeout-mbox)
- [Open a .pst without Outlook](https://hwhang0917.github.io/local-eml/docs/how-to/pst-without-outlook)
- [Free up Gmail storage](https://hwhang0917.github.io/local-eml/docs/how-to/free-up-gmail)

The [full documentation](https://hwhang0917.github.io/local-eml/docs/) also explains [importing](https://hwhang0917.github.io/local-eml/docs/import), [search](https://hwhang0917.github.io/local-eml/docs/search) (including Korean 초성검색), [browsing attachments](https://hwhang0917.github.io/local-eml/docs/attachments), [receiving new mail](https://hwhang0917.github.io/local-eml/docs/sync), [spam and phishing warnings](https://hwhang0917.github.io/local-eml/docs/spam-phishing), [where your data is kept](https://hwhang0917.github.io/local-eml/docs/privacy), [settings and the command line](https://hwhang0917.github.io/local-eml/docs/settings-cli), and [troubleshooting](https://hwhang0917.github.io/local-eml/docs/troubleshooting).

## Contributing

[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) explains how to build and run from source. Please read [`CONTRIBUTING.md`](CONTRIBUTING.md) before opening an issue or pull request. Licenses for the libraries Local Eml uses are listed in the app under **Settings → Attributions**.

## License

[GNU General Public License v3.0](LICENSE)
