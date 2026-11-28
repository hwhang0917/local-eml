# Privacy and your data

## Where your data is kept

Everything is in one folder: `~/.local-eml/` on Linux and macOS, and `%USERPROFILE%\.local-eml\` on Windows.

| Path | What it holds |
|---|---|
| `eml/` | Your messages, as ordinary `.eml` files |
| `db/local-eml.db` | The database: message details, the search index, profiles and settings |
| `logs/local-eml.log` | The log file. It includes subjects and email addresses. Each day's log moves to `local-eml-YYYY-MM-DD.log` at midnight, and logs older than 30 days are deleted |
| `keys/secret.key` | The key used to encrypt saved IMAP passwords, when your system has no secure storage. Otherwise the key is in Credential Manager, the Keychain or the Linux secret service, under `local-eml`, and `keys/` holds only a note saying so |
| `session-token` | The password your browser uses to sign in. See [session token](./settings-cli#session-token) |
| `prefs.json` | Launch preferences (for now, only the temporary sessions switch) |

The folder is created so that other user accounts on the same computer can't open it.

## Network

Local Eml only accepts connections from your own computer (`127.0.0.1`). Other devices on your network can't reach it, and other websites open in your browser can't talk to it. It only connects to the internet when you ask it to: IMAP and S3 imports and exports, and the update check under **Settings → About**.

## Passwords and keys

- The S3 secret key and session token are never saved.
- IMAP passwords are only saved if you turn on background sync for a profile, and then they're encrypted with a key kept in your system's secure storage (Credential Manager, the Keychain or the Linux secret service), or in a file where there is none. See [Receiving new mail](./sync#what-happens-to-your-password).
- Server names, bucket names, usernames and access key IDs are saved as plain text, since they aren't secret.

## Temporary sessions

Use this when you want to open some `.eml` files, look through them, and leave nothing behind.

Turn on **Settings → Privacy → Double-click sessions are temporary**. From the next launch, each double-click starts with an empty library in a temporary folder, and that folder is deleted when you close the app. If the app crashes, the leftover folder is deleted the next time it starts. A banner at the top of the page reminds you that the session is temporary.

The only file kept is `prefs.json`, which remembers that the switch is on. It holds no mail and no passwords.

To keep what you have in a temporary window, turn the switch off from inside it, or tick **Keep the mail from this window** on the same page. When you close the window, its mail, stars, read marks and categories are added to your regular library instead of being deleted. Mail that was already in your library keeps its own marks, and saved profiles come along without their passwords, with background sync off. If the app can't finish this when it closes, it finishes the next time it starts.

You can't use this switch while the background service is running, because the service never closes. On that computer, either stop the service, or start Local Eml with `--ephemeral` (or set `LOCAL_EML_EPHEMERAL=1`). That gives you a temporary session that doesn't touch the service's library.

## Deleting all data

**Settings → Privacy → Delete all data now** deletes every message, the search index, the import history and the logs. It also turns off background sync so the library doesn't fill up again. Your profiles (without passwords), categories, settings and the encryption key are kept. You have to type a confirmation word first.

::: warning No guarantees
Temporary sessions and Delete all data delete files the normal way, the same as if you deleted them yourself. This is not secure erasure. Deleted files can often be recovered, especially on SSDs, and Local Eml does not promise otherwise. Local Eml also can't bring deleted data back, so keep a backup of anything you need. Like the rest of Local Eml, these features come with no warranty, under the [GPL v3](https://github.com/hwhang0917/local-eml/blob/main/LICENSE).
:::
