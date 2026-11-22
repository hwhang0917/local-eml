# Receiving new mail

A saved IMAP profile can fetch new mail on its own. It's opt-in, per profile.

## Turning it on

On the **Import** page, choose **IMAP**. Each saved profile has a card with a switch: turn it on, then click the card and run one import with your password. A green dot on the card means the profile is syncing in the background; amber means it still needs that first import. From then on Local Eml logs in unattended and fetches only messages that arrived since the last sync.

The default interval is every 10 minutes. Change it on the Import page, or set `LOCAL_EML_SYNC_INTERVAL` in the service's environment to a Go duration such as `5m` or `30s`, or `off` to pause. The setting in the app wins over the environment variable.

## What happens to your password

Unattended login needs the password, so turning sync on stores it, encrypted with AES-256-GCM, in the database. The key lives in a separate file, `~/.local-eml/keys/secret.key`, with permissions `0600`. A copied or backed-up database alone cannot reveal the password; an attacker would also need the key file.

Turning sync off removes the stored password and the sync position, so the app asks before it does.

## When it runs

- **Background service:** around the clock, whenever the computer is on.
- **Double-click mode:** only while the window is open.
- **Temporary sessions:** nothing is kept, so there's nothing to sync.

Each sync shows up on the Import page as a job, with the same progress and cancel controls as a manual import.

## Notifications

Tick **Notify me when new mail arrives** under the sync settings to get a desktop notification when a background sync brings in new mail. It is off by default. The browser asks for permission the first time; if you decline, the option stays off, and you can allow notifications later in the browser's site settings.

Notifications come from an open Local Eml tab, so they only appear while the app is open in a browser. The choice is saved per browser. A message flagged as spam or phishing is counted but its sender and subject are not shown.
