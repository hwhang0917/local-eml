# Settings and command line

## Settings

Open **Settings** from the top bar. The book icon at the right of the top bar opens this documentation in a new tab.

| Page | What you can do there |
|---|---|
| **About** | See the version and check for updates. Background service installs can update from here. **Rescan library** adds any `.eml` files in the data folder that the library doesn't know about yet. There are also links to this documentation, GitHub and the issue tracker. |
| **Categories** | Name the colour categories. |
| **Locale** | Choose the interface language, and whether dates show as "Mar 3, 2024" or "2 years ago". |
| **Reading** | Turn on read tracking (off by default). Unread subjects then show in bold, opening a message marks it read, and the list gets an **Unread** filter and a read/unread button on each row. When you first turn it on, you choose whether your existing mail starts as read or unread. After a big import, **Mark all as read** clears the backlog. |
| **Restore from backup** | Bring back stars, categories, settings and profiles from an exported `.zip`. |
| **Duplicates** | Find emails stored more than once, for example after importing a Takeout `.mbox` and then syncing the same account over IMAP, and keep one copy of each. Stars, read state and categories carry over. See [Duplicates](./import#duplicates). |
| **Spam & phishing** | See the messages you've marked, and turn phishing warnings on or off. |
| **Privacy** | Temporary sessions, Delete all data, and where the key for saved IMAP passwords is kept. |
| **Attributions** | Licenses of the open-source libraries Local Eml uses. |

## Command line

You don't need the command line for everyday use. It's here for the background service and for scripting.

```text
local-eml <command> [flags]

  app [--ephemeral]        open in a browser window; stops when the window closes (default)
  serve [--port 7878] [--ephemeral]
                           run the local web server (loopback only)
  install [-y|--yes] [--port 7878]
                           start in the background at sign-in (systemd/launchd/Windows startup)
  uninstall [-y|--yes]     stop and unregister the background service
  token                    print the session token the API and a browser need
  version | -V | --version
```

Running the program with no command is the same as `app`. That's what happens when you double-click it.

If you run `serve` without `--port` and another program is already using 7878, it moves to the next free port, trying up to 7897, and the log says which one it picked. The background service works the same way unless you installed it with a different port. If you do pass `--port`, that port is used as given, and the server stops with an error if it's taken. See [Port 7878 is already in use](./troubleshooting#port-7878-is-already-in-use).

`install` and `uninstall` ask you to confirm first. Add `--yes` to skip the question in scripts.

## Session token

Only your own computer can connect to Local Eml, but that includes every program and every user account on it. So the server also asks for a token: a random password stored in a file called `session-token` in the data folder, which only you can read.

- Double-clicking signs you in automatically. The program gives the browser a one-time sign-in link made from the token, never the token itself. The browser stays signed in for 30 days after you last used it.
- If you open the address in a browser yourself, you'll see a sign-in page the first time. It shows how to open Command Prompt or Terminal and has a **Copy** button for the `local-eml token` command. Run it and paste the token it prints.
- Scripts send it in a header: `curl -H "Authorization: Bearer $(local-eml token)" http://127.0.0.1:7878/api/stats`.

A [temporary session](./privacy#temporary-sessions) gets its own token, and double-clicking handles it for you in the same way.

## Environment variables

| Variable | What it does |
|---|---|
| `LOCAL_EML_PORT` | The port the install script sets up. Default `7878`. |
| `LOCAL_EML_SYNC_INTERVAL` | How often background sync checks for new IMAP mail, for example `5m`, or `off`. A value set in the app takes priority. |
| `LOCAL_EML_EPHEMERAL` | `1` starts a [temporary session](./privacy#temporary-sessions). The background service ignores it. |
| `LOCAL_EML_KEYRING` | `off` keeps the key that encrypts saved IMAP passwords in a file instead of your system's secure storage, and moves it back out if it was there. See [What happens to your password](./sync#what-happens-to-your-password). |
| `LOCAL_EML_LOG_LEVEL` | `debug`, `info`, `warn` or `error`. Default `info`. |

For the background service, set these in the service's own configuration: the systemd unit on Linux, the LaunchAgent plist on macOS, or the Windows service settings.

## Desktop only

The window needs to be at least 640 pixels wide, so a vertical monitor or a window snapped to half the screen works fine. In a narrower window some list columns step aside: the library drops the size column and shows dates without the time (hover a date for the full one), and the attachments list drops the email column first, then the type and size columns. If the window is narrower than 640 pixels, you'll see a message asking you to widen it. Phones can't connect anyway, since only your own computer can reach Local Eml.
