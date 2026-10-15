# Receiving new mail

A saved IMAP profile can fetch new mail on its own. It's opt-in, per profile.

## Turning it on

On the **Import** page, select a saved IMAP profile and enable **Receive new mail in the background**, then run one import with your password. From then on Local Eml logs in unattended and fetches only messages that arrived since the last sync.

The default interval is every 10 minutes. Change it on the Import page, or set `LOCAL_EML_SYNC_INTERVAL` in the service's environment to a Go duration such as `5m` or `30s`, or `off` to pause. The setting in the app wins over the environment variable.

## What happens to your password

Unattended login needs the password, so turning sync on stores it, encrypted with AES-256-GCM, in the database. The key lives in a separate file, `~/.local-eml/keys/secret.key`, with permissions `0600`. A copied or backed-up database alone cannot reveal the password; an attacker would also need the key file.

Turning sync off removes the stored password and the sync position on the next save.

## When it runs

- **Background service:** around the clock, whenever the computer is on.
- **Double-click mode:** only while the window is open.
- **Temporary sessions:** nothing is kept, so there's nothing to sync.

Each sync shows up on the Import page as a job, with the same progress and cancel controls as a manual import.
