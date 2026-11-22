# Install

Local Eml is a single program file. You can run it in three ways:

| | Double-click | Background service | winget |
|---|---|---|---|
| Needs a terminal | No | Once, to install | Once, to install |
| Starts with the computer | No | Yes | No |
| Fetches new IMAP mail | While the window is open | All the time | While the window is open |

If you're not sure, use the first one.

## Download and double-click

Download the file for your system from the [latest release](https://github.com/hwhang0917/local-eml/releases/latest) and open it. Local Eml opens in its own browser window. A few minutes after you close the window, the program quits, and nothing is left running.

**Windows:** download `local-eml-windows-amd64.exe` (or `-arm64` for ARM laptops). The first time you open it, Windows may say **"Windows protected your PC"**. It does this for any new program it hasn't seen many people use. Click **More info**, then **Run anyway**.

**macOS:** download `local-eml-darwin-arm64.app.zip` for Apple silicon Macs or `local-eml-darwin-amd64.app.zip` for Intel Macs. Unzip it and double-click **Local Eml**. macOS will say it can't verify the developer, because the app isn't from the App Store and isn't notarized by Apple. Open **System Settings → Privacy & Security**, scroll down to the message about Local Eml, click **Open Anyway**, and then open the app again. On macOS 14 or earlier, right-click the app and choose **Open** instead.

**Linux:** download `local-eml-linux-amd64` or `-arm64`, make it executable with `chmod +x`, and run it. It opens in your default browser.

When you run it this way, new mail is only fetched while the window is open. If the background service is already running, double-clicking opens a window onto the service instead of starting a second copy.

If you want each double-click to start empty and leave nothing behind, see [temporary sessions](./privacy#temporary-sessions).

## Background service (Linux, macOS, Windows)

**Linux / macOS**

```bash
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.sh | sh
```

**Windows (PowerShell)**

```powershell
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/install.ps1 | iex
```

The script downloads the latest release, checks that the file wasn't corrupted or altered, and sets Local Eml up to start automatically. When it's done, open <http://localhost:7878>. The first time, the page asks for a session token. The page walks you through getting one: open Command Prompt (Windows) or Terminal (macOS), copy the command with the **Copy** button, paste it there and press Enter, then paste the token it prints back into the page. Double-clicking the program also works and signs you in automatically. See [session token](./settings-cli#session-token) for why this is needed.

To use a port other than 7878, set `LOCAL_EML_PORT` before running the script, for example `LOCAL_EML_PORT=9000 curl … | sh`, or `$env:LOCAL_EML_PORT=9000` in PowerShell. Whichever port you choose, only your own computer can connect to it.

## winget (Windows)

```powershell
winget install hwhang0917.LocalEml
```

This installs the program and adds the `local-eml` command. You update it with `winget upgrade`, so the app's own updater is turned off for copies installed this way.

::: info
The winget listing is still waiting for review. Until it's approved, use one of the methods above.
:::

## Updating

- Background service: go to **Settings → About**, check for updates, and install. The service restarts on its own.
- Double-click: download the new file from the releases page and use it instead of the old one.
- winget: run `winget upgrade hwhang0917.LocalEml`.

## Checking a download

You can confirm that a file really came from this project's release process. Each release file has a [build provenance attestation](https://docs.github.com/en/actions/security-for-github-actions/using-artifact-attestations), which you can check with the GitHub CLI:

```bash
gh attestation verify local-eml-linux-amd64 --repo hwhang0917/local-eml
```

Each release also includes a `SHA256SUMS` file listing the checksums of all its files.

## Uninstall

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh

# Windows
irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1 | iex
```

This removes the program and the service but keeps your mail. To delete your mail and settings as well, use these instead:

```bash
# Linux / macOS
curl -fsSL https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.sh | sh -s -- --purge
```

```powershell
# Windows
& ([scriptblock]::Create((irm https://raw.githubusercontent.com/hwhang0917/local-eml/main/scripts/uninstall.ps1))) -Purge
```
