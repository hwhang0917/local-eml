# Troubleshooting

## A red banner says the server isn't responding

The page is open but the background process has stopped. For a service install, restart it:

```bash
# Linux
systemctl --user restart local-eml
# macOS
launchctl kickstart -k gui/$(id -u)/local-eml
```

On Windows, restart the **Local Eml** service from Services. For double-click mode, just open the executable again.

## The page says "Widen the window to continue"

The window is narrower than the interface needs. Make it wider or maximise it. If you see this when printing, update to v0.15.1 or later, where printing no longer triggers it.

## Port 7878 is already in use

Install with a different port: set `LOCAL_EML_PORT` before running the installer. For a manual run, `local-eml serve --port 9000`.

## Double-clicking opens the service instead of a separate window

That's by design. If the service is running, a double-click opens a window onto it rather than starting a second copy. To get an independent session that doesn't touch the service's library, use a [temporary session](./privacy#temporary-sessions).

## The updater says to use winget

The copy you're running was installed with winget, which tracks its own version. Update with:

```powershell
winget upgrade hwhang0917.LocalEml
```

## A message shows a phishing warning but it's legitimate

The warning is a heuristic. Choose **Not spam** on the banner to silence it for that message, or turn warnings off under **Settings → Spam & phishing**. See [Spam and phishing](./spam-phishing).

## Finding the logs

```bash
tail -f ~/.local-eml/logs/local-eml.log
```

On Windows: `%USERPROFILE%\.local-eml\logs\local-eml.log`. Set `LOCAL_EML_LOG_LEVEL=debug` in the service environment for more detail. Logs include subjects and addresses, so redact before sharing.

## Reporting a bug

Open an issue at [github.com/hwhang0917/local-eml/issues](https://github.com/hwhang0917/local-eml/issues) with your version (**Settings → About**), operating system, how you run it, and the relevant log lines.
