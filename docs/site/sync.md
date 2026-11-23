# Receiving new mail

Local Eml can check a saved IMAP profile for new mail on its own. This is off until you turn it on, and you turn it on separately for each profile.

## Turning it on

On the **Import** page, choose **IMAP**. Each saved profile appears as a card with a switch. Turn the switch on, then click the card and run one import with your password. After that, Local Eml signs in by itself and downloads only the messages that arrived since it last checked.

The dot on each card shows where things stand. Green means the profile is syncing in the background. Amber means sync is on but still waiting for that first import with your password.

By default it checks every 10 minutes. You can change that with **Check interval** on the Import page, anywhere from every minute to once an hour. If you run the background service, you can also set `LOCAL_EML_SYNC_INTERVAL` in its environment to a value like `5m` or `30s`, or `off` to pause it. If both are set, the value in the app is used.

## Instant delivery

Most mail servers, including Gmail, Outlook, iCloud and Naver, can tell a connected app the moment a message lands (this is called IMAP IDLE). When a profile's server supports it, Local Eml keeps one connection open to it and fetches new mail within a second or two, instead of waiting for the next check. Think of it as the mail carrier ringing the doorbell, rather than you walking to the mailbox every 10 minutes.

The card shows which one you get:

- **Live**: the server announces new mail and Local Eml fetches it straight away.
- **Reconnecting**: the connection dropped, for example because the network changed. Local Eml reconnects by itself, waiting a little longer after each failed try, and fetches anything that arrived while it was away.
- **Checked on interval**: the server can't announce new mail, so it's checked every interval as before.

Only the profile's folder is watched, usually INBOX. The regular check keeps running beside it as a safety net, and pausing the check interval pauses instant delivery too.

When new mail comes in, an open **Library** page updates by itself. The new rows are tinted for a moment so you can spot them.

## What happens to your password

To sign in without you, Local Eml has to keep your password. When you turn sync on, it saves the password in the database, encrypted with AES-256-GCM. The key is kept in a separate file, `~/.local-eml/keys/secret.key`, that only your user account can read. Someone who gets a copy of the database alone, for example from a backup, still can't read the password without that key file.

Turning sync off deletes the stored password and forgets where the last check stopped, so the app asks you to confirm first. To turn it back on, run one import with the password again.

## When it runs

- Background service: whenever the computer is on.
- Double-click: only while the window is open.
- Temporary sessions: never, because nothing is kept between launches.

Each check appears on the Import page as a job. You can watch its progress and cancel it like any other import.

## Notifications

If you'd like a desktop notification when new mail comes in, tick **Notify me when new mail arrives** under the sync settings. It's off by default. The first time, your browser asks for permission. If you say no, the option stays off. You can allow notifications later in your browser's settings for the site and then tick it again.

Notifications come from the Local Eml tab, so you only get them while Local Eml is open in your browser. The setting is remembered separately in each browser. Messages flagged as spam or phishing are included in the count, but their sender and subject aren't shown.

If notifications are on but don't pop up, see [New-mail notifications don't pop up](./troubleshooting#new-mail-notifications-don-t-pop-up).
