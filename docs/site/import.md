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

## IMAP mailbox

IMAP is the standard way mail apps read a mailbox on a server. Enter the server address (host), port, username, password and folder. Most providers use port 993, which is encrypted (TLS). Port 143 also works and switches to encryption with STARTTLS. The folder is `INBOX` unless you change it.

Local Eml only reads. It downloads copies and never moves, marks or deletes anything on the server.

You can save the connection details as a **profile** so you don't have to type them again. The password isn't saved; you type it each time. The only exception is if you turn on background sync for that profile. See [Receiving new mail](./sync).

## AWS S3 bucket

Enter the bucket, region, an optional key prefix, the access key ID and the secret key. The secret key and session token are never saved, so you enter them each time. Everything else can be saved as a profile.

## Exporting

The **Export** page saves your whole library as one `.zip` file, or uploads it to an S3 bucket. Files that are already in the bucket are skipped, so you can run the same export again safely.

The `.zip` also contains a copy of the database, minus any saved passwords. After you import the `.eml` files from it, for example on a new computer, **Settings → Restore from backup** brings back your stars, categories, settings and profiles.
