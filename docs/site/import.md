# Importing mail

All mail comes in through the **Import** page. Local Eml recognises messages it already has and skips them, so importing the same thing twice does no harm. You can cancel an import at any time.

![The import page](https://hwhang0917.github.io/local-eml/img/en/import.webp)

## Files and folders

Drag any of these onto the page, or click to choose them:

- `.eml` files, one at a time or a whole folder
- `.zip` files that contain `.eml` files
- `.mbox` files, like the ones Google Takeout, Thunderbird and Apple Mail export
- Outlook `.pst` files

Whatever the format, each message is saved in your library as its own `.eml` file.

## Duplicates

Local Eml skips a message only when the file is exactly the same. The same email can still come in twice by two different routes, because each route stores it a little differently. For example, a Google Takeout `.mbox` adds a label header that IMAP doesn't.

**Settings → Duplicates** finds these copies by their Message-ID and date, shows how much space the extra copies use, and keeps one copy of each. You can pick which copy to keep, or remove every extra copy at once, in which case the oldest import stays. Whatever you did to any copy (a star, read state, a category) moves to the copy that stays.

## IMAP mailbox

IMAP is the standard way mail apps read a mailbox on a server. Enter the server address (host), port, username, password and folder. Most providers use port 993, which is encrypted (TLS). Port 143 also works and switches to encryption with STARTTLS. The folder is `INBOX` unless you change it.

Local Eml only reads. It downloads copies and never moves, marks or deletes anything on the server.

You can save the connection details as a **profile** so you don't have to type them again. The password isn't saved; you type it each time. The only exception is if you turn on background sync for that profile. See [Receiving new mail](./sync).

## AWS S3 bucket

Enter the bucket, region, an optional key prefix, the access key ID and the secret key. The secret key and session token are never saved, so you enter them each time. Everything else can be saved as a profile.

## Exporting

The **Export** page saves your whole library as one `.zip` file, or uploads it to an S3 bucket. Files that are already in the bucket are skipped, so you can run the same export again safely.

The `.zip` also contains a copy of the database, minus any saved passwords. After you import the `.eml` files from it, for example on a new computer, **Settings → Restore from backup** brings back your stars, categories, settings and profiles.

## Automatic backup

If you've deleted mail from your mailbox after archiving it, Local Eml may hold the only copy. **Automatic backup** at the bottom of the **Export** page copies your library into a folder on a schedule, so one dead disk can't take it with it.

1. Enter the full path of a folder on an external drive, a NAS, or a folder that Dropbox, OneDrive or Google Drive syncs, for example `E:\Mail Backup` or `/Volumes/Backup/Mail`. Local Eml creates the folder if it doesn't exist.
2. Choose **Daily** or **Weekly** and click **Save**. **Back up now** runs one straight away.

Backups run while Local Eml is running: all the time if you installed it as a background service, or while its window is open otherwise. A backup that was due while it was closed runs a couple of minutes after it next starts. The first backup copies everything. After that, each backup copies only new mail, so it usually takes seconds. Nothing is ever deleted from the backup folder, so mail you delete in Local Eml stays in the backup. If a backup fails, for example because the drive isn't plugged in, the card shows why and when the last good backup was, and Local Eml tries again an hour later.

The folder holds an `emails` folder with one `.eml` file per message, and `local-eml.db` with your stars, categories, settings and profiles (without passwords). To restore on a new computer, drop the `emails` folder onto the **Import** page, then choose `local-eml.db` under **Settings → Restore from backup**.
