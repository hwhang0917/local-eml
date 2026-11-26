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

Local Eml moves to the next free port on its own (7879, then 7880, and so on), and the log says which one it picked. Double-clicking the program still finds it there. If another copy of Local Eml already holds the port, it doesn't start a second one.

To keep a fixed address instead, reinstall with a different port by setting `LOCAL_EML_PORT` before running the install script (see [Install](./install#background-service-linux-macos-windows)). If you start the server yourself, use `local-eml serve --port 9000`. A port you choose this way is never changed: if it's taken, the server stops with an error.

## Double-clicking opens the service instead of a new window

That's on purpose. When the background service is running, double-clicking opens a window onto it instead of starting a second copy. If you want a separate session that doesn't touch the service's library, use a [temporary session](./privacy#temporary-sessions).

## The updater says to use winget

You installed this copy with winget, so winget handles its updates. Run:

```powershell
winget upgrade hwhang0917.LocalEml
```

## A normal message shows a phishing warning

The checks can't be right every time. Choose **Not spam** on the banner to hide the warning for that message, or turn warnings off under **Settings → Spam & phishing**. See [Spam and phishing](./spam-phishing).

## New-mail notifications don't pop up

First check that **Notify me when new mail arrives** is ticked under the sync settings and that Local Eml is open in a browser tab. The site doesn't need HTTPS for this: browsers treat `http://localhost` as secure, so notifications work at `http://localhost:7878`.

If the option is on but nothing pops up, your operating system is usually holding the notifications back. Click the **(?)** next to the option for these steps in the app, and use **Send a test notification** there to check your settings. The setting is under your browser's name (Google Chrome, Microsoft Edge, Safari, Firefox), not Local Eml, because the browser shows the notification.

**Windows**

1. Open **Settings → System → Notifications** and turn off **Do not disturb**.
2. On the same page, open **Turn on do not disturb automatically** and clear the rules you don't want, such as *when duplicating your display*, *when playing a game*, or *when using an app in full-screen mode*.
3. Under **Notifications from apps and other senders**, click your browser and turn on **Show notification banners**. If notifications appear in the list (Windows key + N) but never pop up in the corner, this is usually why.

**macOS**

1. Open **System Settings → Notifications**, pick your browser, and turn on **Allow notifications**. Chrome may be listed as *Google Chrome Helper (Alerts)*.
2. On the same screen, set the alert style to **Banners** or **Alerts**. With **None**, notifications only collect in Notification Center.
3. If the Focus icon (a moon) is in the menu bar, notifications are held back. Turn Focus off in Control Center, or add your browser to the allowed apps under **System Settings → Focus**.

On Linux, check your desktop's Do Not Disturb setting and its notification settings for your browser.

## Finding the log file

On Linux and macOS:

```bash
tail -F ~/.local-eml/logs/local-eml.log
```

`-F` keeps following the log after it rotates at midnight. Earlier days are in `local-eml-YYYY-MM-DD.log` next to it, kept for 30 days. On Windows, the file is `%USERPROFILE%\.local-eml\logs\local-eml.log`. For more detail, set `LOCAL_EML_LOG_LEVEL=debug` in the service's environment. The log includes subjects and email addresses, so remove anything private before you share it.

## Reporting a bug

Open an issue at [github.com/hwhang0917/local-eml/issues](https://github.com/hwhang0917/local-eml/issues). Include your version (shown under **Settings → About**), your operating system, whether you use the double-click version or the background service, and the relevant lines from the log.
