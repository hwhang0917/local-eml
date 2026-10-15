# How to open a .pst file without Outlook

A `.pst` file is Outlook's own mailbox archive. Opening one normally needs Outlook installed and an account to attach it to. Local Eml reads `.pst` files directly, so you can browse an old archive on any computer, including macOS and Linux.

## Where .pst files come from

- Outlook's **File → Open & Export → Import/Export → Export to a file → Outlook Data File (.pst)**.
- Archives created by Outlook's AutoArchive feature, usually in `Documents\Outlook Files`.
- Mailbox exports handed over by an IT department or a previous employer.

## Open it in Local Eml

1. Open Local Eml and go to **Import**.
2. Drag the `.pst` file onto the page, or pick it with the file chooser.
3. Wait for the import to finish. Every message, with its attachments, becomes a plain `.eml` file in the library.

Folder structure inside the `.pst` isn't preserved as folders; messages are listed together, and you find them by search, date range, sender or by assigning categories.

## What doesn't work

- **`.ost` files** are Outlook's cache of a live mailbox, not an archive, and aren't supported. Export a `.pst` from Outlook instead.
- **Outlook.com accounts over IMAP.** Microsoft no longer allows password sign-in to Outlook.com mailboxes, so Local Eml's IMAP import can't connect to them. Exporting a `.pst` from the Outlook desktop app is the reliable path.

If a `.pst` fails to import, open an [issue](https://github.com/hwhang0917/local-eml/issues) with the error from the import log; don't attach the file itself.
