# Privacy and your data

## Where your data lives

Everything is in one folder: `~/.local-eml/` on Linux and macOS, `%USERPROFILE%\.local-eml\` on Windows.

| Path | Contents |
|---|---|
| `eml/` | Your messages as plain `.eml` files, named by content hash |
| `db/local-eml.db` | The SQLite database: metadata, search index, profiles, settings |
| `logs/local-eml.log` | The application log, which includes subjects and addresses |
| `keys/secret.key` | The key that encrypts stored IMAP passwords |
| `prefs.json` | Launch preferences, currently just the temporary-sessions switch |

The folder and everything in it are created with permissions that keep other users on the machine out.

## Network

Local Eml listens only on `127.0.0.1`. Nothing is reachable from your network, and requests from other websites are refused. The only outbound connections are the ones you ask for: IMAP and S3 imports or exports, and the update check under **Settings → About**.

## Credentials

- **S3 secret key and session token** are never saved.
- **IMAP passwords** are not saved unless you turn on background sync for a profile. Then they're encrypted at rest; see [Receiving new mail](./sync#what-happens-to-your-password).
- Host names, bucket names, usernames and access key IDs are saved in plain form, since they aren't secrets.

## Temporary sessions

For when you just want to open a folder of `.eml` files, search it, and leave nothing behind.

Turn on **Settings → Privacy → Double-click sessions are temporary**. From the next launch, every double-click starts with an empty library in a throwaway folder under the system temp directory, and that folder is deleted when the app closes. If the app crashes, the leftovers are removed the next time it starts. A banner at the top of the page shows while a temporary session is running.

The only thing kept on disk is `prefs.json`, which remembers the switch. It contains no mail or credentials.

The switch is unavailable while the background service is running, because the service never closes. On those machines, stop the service, or start Local Eml with `--ephemeral` or `LOCAL_EML_EPHEMERAL=1`, which gives a temporary session that never touches the service's library.

## Deleting all data

**Settings → Privacy → Delete all data now** removes every message, the search index, import history and logs, and turns off background sync so the library doesn't refill. Saved profiles without their passwords, categories, settings and the encryption key are kept. You type a confirmation word before it runs.

::: warning No guarantees
Temporary sessions and Delete all data remove files the ordinary way, the same as deleting them yourself. They are a convenience, not secure erasure: Local Eml does not guarantee that deleted data can't be recovered, and on SSDs and journaling file systems it often can be. Local Eml also can't bring deleted data back, so keep a backup of anything you need. Like the rest of Local Eml, these features are provided as is, without warranty of any kind, under the [GPL v3](https://github.com/hwhang0917/local-eml/blob/main/LICENSE).
:::
