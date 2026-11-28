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

You can save the connection details as a **profile** so you don't have to type them again. Saved profiles show up as cards at the top of the IMAP tab, and clicking one fills in the form. The password isn't saved; you type it each time. The only exception is if you turn on background sync for that profile with the switch on its card. See [Receiving new mail](./sync).

## S3 bucket

Local Eml imports every `.eml` file it finds under a prefix in a bucket. Enter the bucket, region, an optional key prefix, the access key ID and the secret key. If you leave both keys blank, it uses the credentials already set up on your computer, in `~/.aws/credentials` or the `AWS_*` environment variables. The secret key and session token are never saved, so you enter them each time.

### MinIO, Cloudflare R2, Backblaze B2 and others

Amazon isn't the only company that offers S3 storage. MinIO, Cloudflare R2, Backblaze B2, Wasabi and many home NAS boxes understand the same commands, so Local Eml works with them too.

In the **S3-compatible service** box, enter the service's address as the **Endpoint URL**. For example, R2 looks like `https://<account-id>.r2.cloudflarestorage.com`, and a MinIO server on your home network might be `http://nas.local:9000`. Leave it blank for AWS.

For MinIO and most servers you run yourself, also turn on **Path-style addressing**. It changes where the bucket name goes in the address: `nas.local:9000/mail` instead of `mail.nas.local:9000`. If a self-hosted server says the bucket doesn't exist or the connection fails, this switch is the usual fix. When the region is left empty, `us-east-1` is used, which is what most of these services expect.

### Saved profiles

Everything except the secret key can be saved as a profile. Saved profiles appear as cards above the form, on both the **Import** and **Export** pages, so one profile works for both. Each card shows the bucket, prefix, region or endpoint, and whether it uses a saved access key ID or your computer's own credentials. Click a card to fill in the form, then type the secret key.

Deleting a profile only forgets the saved details. Nothing in the bucket is touched.

## Exporting

The **Export** page saves your whole library as one `.zip` file, or uploads it to an S3 bucket, on AWS or any of the S3-compatible services above. Files that are already in the bucket are skipped, so you can run the same export again safely.

To export only some messages, such as one search's results, use the export buttons in the library. See [Exporting search results](./search#exporting-search-results).

The `.zip` also contains a copy of the database, minus any saved passwords. After you import the `.eml` files from it, for example on a new computer, **Settings → Restore from backup** brings back your stars, categories, settings and profiles.

## Automatic backup

If you've deleted mail from your mailbox after archiving it, Local Eml may hold the only copy. **Automatic backup** at the bottom of the **Export** page copies your library into a folder on a schedule, so one dead disk can't take it with it.

1. Enter the full path of a folder on an external drive, a NAS, or a folder that Dropbox, OneDrive or Google Drive syncs, for example `E:\Mail Backup` or `/Volumes/Backup/Mail`. Local Eml creates the folder if it doesn't exist.
2. Choose **Daily** or **Weekly** and click **Save**. **Back up now** runs one straight away.

Backups run while Local Eml is running: all the time if you installed it as a background service, or while its window is open otherwise. A backup that was due while it was closed runs a couple of minutes after it next starts. The first backup copies everything. After that, each backup copies only new mail, so it usually takes seconds. Nothing is ever deleted from the backup folder, so mail you delete in Local Eml stays in the backup. If a backup fails, for example because the drive isn't plugged in, the card shows why and when the last good backup was, and Local Eml tries again an hour later.

The folder holds an `emails` folder with one `.eml` file per message, and `local-eml.db` with your stars, categories, settings and profiles (without passwords). To restore on a new computer, drop the `emails` folder onto the **Import** page, then choose `local-eml.db` under **Settings → Restore from backup**.
