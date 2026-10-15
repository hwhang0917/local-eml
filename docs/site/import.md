# Importing mail

Everything comes in through the **Import** page. Duplicates are detected by file hash and skipped, so importing the same source twice is harmless. An import in progress can be cancelled at any time.

![The import page](https://hwhang0917.github.io/local-eml/img/en/import.webp)

## Files and folders

Drop any of these onto the page or pick them with the file chooser:

- Single `.eml` files, or a folder of them.
- `.zip` archives containing `.eml` files.
- `.mbox` files, as exported by Google Takeout, Thunderbird or Apple Mail.
- Outlook `.pst` files.

Each message becomes a plain `.eml` file in your library, whatever format it came from.

## IMAP mailbox

Enter the host, port, username, password and folder. Port 993 connects with TLS; port 143 uses STARTTLS. The folder defaults to `INBOX`.

Access is strictly read-only. Local Eml fetches copies and never moves, flags or deletes anything on the server.

Save the connection as a **profile** to avoid retyping it. By default the password is not saved; you enter it each time. It is stored only if you turn on background sync for the profile, see [Receiving new mail](./sync).

## AWS S3 bucket

Enter the bucket, region, optional key prefix, access key ID and secret key. The secret key and session token are never saved; you enter them per import. Everything else can be saved as a profile.

## Exporting

The **Export** page writes your whole library to a single `.zip`, or uploads it to an S3 bucket. Keys that already exist in the destination are skipped, so re-running an export is safe.

The `.zip` includes a snapshot of the database without stored passwords. **Settings → Restore from backup** can read it back to recover stars, categories, settings and profiles after importing the `.eml` files.
