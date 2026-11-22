# How to open a .pst file without Outlook

A `.pst` file is an Outlook mailbox saved as a file. Normally you need Outlook installed to open one. Local Eml can read `.pst` files itself, so you can look through an old archive on any computer, including a Mac or a Linux machine.

## Where .pst files come from

- Exporting from Outlook: **File → Open & Export → Import/Export → Export to a file → Outlook Data File (.pst)**.
- Outlook's AutoArchive feature, which usually saves them in `Documents\Outlook Files`.
- An IT department or former employer giving you a copy of your mailbox.

## Open it in Local Eml

1. Open Local Eml and go to **Import**.
2. Drag the `.pst` file onto the page, or click to choose it.
3. Wait for the import to finish. Every message, with its attachments, is saved as an `.eml` file in your library.

The folders inside the `.pst` aren't kept. All messages appear in one list, and you find them by searching, by date, or by sender. You can also sort them yourself with categories.

## What doesn't work

- **`.ost` files.** These are Outlook's temporary copy of a live mailbox, not an archive, and Local Eml can't read them. Export a `.pst` from Outlook instead.
- **Outlook.com accounts over IMAP.** Microsoft no longer lets apps sign in to Outlook.com mailboxes with a password, so Local Eml's IMAP import can't connect. Export a `.pst` from the Outlook desktop app instead.

If a `.pst` won't import, [open an issue](https://github.com/hwhang0917/local-eml/issues) with the error message from the import. Don't attach the file itself, since it contains your mail.
