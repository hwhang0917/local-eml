# Settings and command line

## Settings pages

| Page | What's there |
|---|---|
| **About** | Version, update check, and in-place update for service installs |
| **Categories** | Name the fixed set of colour categories |
| **Locale** | Interface language, and absolute or relative dates |
| **Reading** | Read tracking: off by default. When on, unread subjects are bold, opening a message marks it read, and the list gets an **Unread** filter and a per-row toggle. The first time you turn it on you choose whether existing mail starts read or unread; **Mark all as read** catches up after a big import |
| **Restore from backup** | Read stars, categories, settings and profiles back from an exported `.zip` |
| **Spam & phishing** | Flagged messages, and the switch for phishing warnings |
| **Privacy** | Temporary sessions and Delete all data |
| **Attributions** | Licenses of the open-source libraries used |

## Command line

```text
local-eml <command> [flags]

  app [--ephemeral]        open in a browser window; stops when the window closes (default)
  serve [--port 7878] [--ephemeral]
                           run the local web server (loopback only)
  install [-y|--yes] [--port 7878]
                           register as a background service (systemd/launchd/Windows Service)
  uninstall [-y|--yes]     stop and unregister the background service
  version | -V | --version
```

Running the executable with no arguments is the same as `app`; that's what a double-click does.

`install` and `uninstall` ask for confirmation. `--yes` skips the prompt for scripts.

## Environment variables

| Variable | Effect |
|---|---|
| `LOCAL_EML_PORT` | Port for the installer scripts to register. Default `7878`. |
| `LOCAL_EML_SYNC_INTERVAL` | IMAP background poll interval, e.g. `5m`, or `off`. The value saved in the app wins. |
| `LOCAL_EML_EPHEMERAL` | `1` starts a [temporary session](./privacy#temporary-sessions). Ignored by the service. |
| `LOCAL_EML_LOG_LEVEL` | `debug`, `info`, `warn` or `error`. Default `info`. |

For a service, set these in the service's environment: the systemd unit, the LaunchAgent plist, or the Windows service configuration.

## Desktop only

The interface needs a window at least about 768 pixels wide. Below that it shows a notice asking for a wider window instead of a broken layout. Since the server only listens on your own machine, phones can't reach it anyway.
