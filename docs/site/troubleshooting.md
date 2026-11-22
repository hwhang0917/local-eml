# Troubleshooting

## A red banner says the server isn't responding

The page is still open, but the program behind it has stopped. If you installed the background service, restart it:

```bash
# Linux
systemctl --user restart local-eml
# macOS
launchctl kickstart -k gui/$(id -u)/local-eml
```

On Windows, open **Services**, find **Local Eml**, and restart it. If you use the double-click version, just open the program again.

## The page says "Widen the window to continue"

The window is too narrow. Make it wider or maximise it. If this happens when you print, update to v0.15.1 or later, which fixes it.

## Port 7878 is already in use

Another program is using that port. Reinstall with a different one by setting `LOCAL_EML_PORT` before running the install script (see [Install](./install#background-service-linux-macos-windows)). If you start the server yourself, use `local-eml serve --port 9000`.

## Double-clicking opens the service instead of a new window

That's on purpose. When the background service is running, double-clicking opens a window onto it instead of starting a second copy. If you want a separate session that doesn't touch the service's library, use a [temporary session](./privacy#temporary-sessions).

## The updater says to use winget

You installed this copy with winget, so winget handles its updates. Run:

```powershell
winget upgrade hwhang0917.LocalEml
```

## A normal message shows a phishing warning

The checks can't be right every time. Choose **Not spam** on the banner to hide the warning for that message, or turn warnings off under **Settings → Spam & phishing**. See [Spam and phishing](./spam-phishing).

## Finding the log file

On Linux and macOS:

```bash
tail -f ~/.local-eml/logs/local-eml.log
```

On Windows, the file is `%USERPROFILE%\.local-eml\logs\local-eml.log`. For more detail, set `LOCAL_EML_LOG_LEVEL=debug` in the service's environment. The log includes subjects and email addresses, so remove anything private before you share it.

## Reporting a bug

Open an issue at [github.com/hwhang0917/local-eml/issues](https://github.com/hwhang0917/local-eml/issues). Include your version (shown under **Settings → About**), your operating system, whether you use the double-click version or the background service, and the relevant lines from the log.
