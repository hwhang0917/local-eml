# Install

Local Eml is a single executable. There are three ways to run it; pick the one that fits how you'll use it.

| | Double-click | Background service | winget |
|---|---|---|---|
| Needs a terminal | No | Once, to install | Once, to install |
| Starts with the computer | No | Yes | No |
| Receives new IMAP mail around the clock | Only while the window is open | Yes | Only while the window is open |

## Download and double-click

The simplest way, and the one most people want. Download the file for your OS from the [latest release](https://github.com/hwhang0917/local-eml/releases/latest) and open it. Local Eml opens in its own browser window. When you close the window, it shuts itself down a few minutes later, and nothing keeps running in the background.

**Windows:** `local-eml-windows-amd64.exe` (or `-arm64` on ARM laptops). The first time, Windows may show **"Windows protected your PC"**. That appears for any new program that isn't yet known to Microsoft's reputation service. Click **More info**, then **Run anyway**.

**macOS:** `local-eml-darwin-arm64` for Apple silicon, `local-eml-darwin-amd64` for Intel. macOS blocks programs from outside the App Store that aren't notarized, with a message that it can't verify the developer. Open **System Settings → Privacy & Security**, scroll down to the message about Local Eml, click **Open Anyway**, then open the file again. On macOS 14 and earlier, right-click the file and choose **Open** instead.

**Linux:** `local-eml-linux-amd64` or `-arm64`. Mark it executable with `chmod +x`, then run it. It opens in your default browser.

In this mode new mail is only fetched while the window is open. If the background service is already running, double-clicking just opens a window onto it.

Want each double-click to start fresh and leave nothing behind? See [temporary sessions](./privacy#temporary-sessions).

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
