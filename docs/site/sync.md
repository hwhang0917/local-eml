# Receiving new mail

Local Eml can check a saved IMAP profile for new mail on its own. This is off until you turn it on, and you turn it on separately for each profile.

## Turning it on

On the **Import** page, pick a saved IMAP profile and turn on **Receive new mail in the background**. Then run one import with your password. After that, Local Eml signs in by itself and downloads only the messages that arrived since it last checked.

It checks every 10 minutes. You can change that on the Import page. If you run the background service, you can also set `LOCAL_EML_SYNC_INTERVAL` in its environment to a value like `5m` or `30s`, or `off` to pause it. If both are set, the value in the app is used.

## What happens to your password

To sign in without you, Local Eml has to keep your password. When you turn sync on, it saves the password in the database, encrypted with AES-256-GCM. The key is kept in a separate file, `~/.local-eml/keys/secret.key`, that only your user account can read. Someone who gets a copy of the database alone, for example from a backup, still can't read the password without that key file.

When you turn sync off and save, the stored password is deleted.

## When it runs

- Background service: whenever the computer is on.
- Double-click: only while the window is open.
- Temporary sessions: never, because nothing is kept between launches.

Each check appears on the Import page as a job. You can watch its progress and cancel it like any other import.
