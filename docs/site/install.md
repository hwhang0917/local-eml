# Install

Local Eml is a single executable. There are three ways to run it; pick the one that fits how you'll use it.

| | Background service | Double-click | winget |
|---|---|---|---|
| Starts with the computer | Yes | No | No |
| Receives new IMAP mail around the clock | Yes | Only while the window is open | Only while the window is open |
| Needs a terminal | Once, to install | No | Once, to install |

## Background service (Linux, macOS, Windows)

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

The installer downloads the latest release, verifies its checksum, and registers Local Eml as a background service that starts automatically. Then open <http://localhost:7878>.

To use a different port, set `LOCAL_EML_PORT` before running the installer, for example `LOCAL_EML_PORT=9000 curl … | sh`, or `$env:LOCAL_EML_PORT=9000` in PowerShell. Whatever the port, it only listens on your own machine.

## Double-click, no install

Download the file for your OS from the [latest release](https://github.com/hwhang0917/local-eml/releases/latest) and double-click it. Local Eml opens in its own browser window. When you close the window, it shuts itself down a few minutes later, and nothing keeps running in the background.

In this mode new mail is only fetched while the window is open. If the background service is already running, double-clicking just opens a window onto it.

Want each double-click to start fresh and leave nothing behind? See [temporary sessions](./privacy#temporary-sessions).

## winget (Windows)

```powershell
winget install hwhang0917.LocalEml
```

This installs the portable executable and adds the `local-eml` command. Updates come through `winget upgrade`; the in-app updater steps aside for copies installed this way.

::: info
The winget listing is pending review in the community repository. Until it's accepted, use one of the methods above.
:::

## Updating

- **Service installs** check for updates under **Settings → About** and can install them in place. The service restarts itself.
- **Double-click** users download the new file from the releases page.
- **winget** users run `winget upgrade hwhang0917.LocalEml`.

## Verifying a download

Every release asset carries a [build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations) proving it was built by this repository's release workflow from a tagged commit:

```bash
gh attestation verify local-eml-linux-amd64 --repo hwhang0917/local-eml
```

`SHA256SUMS` in each release lists the checksums of the same files.

## Uninstall

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

Add `--purge` (or `-Purge` on Windows) to also delete the data folder. Without it, your mail library stays on disk.
