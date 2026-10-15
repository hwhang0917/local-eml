# How to free up Gmail storage without deleting mail

Gmail shares its storage with Drive and Photos, and old mail is usually the biggest piece. Local Eml lets you keep a complete, searchable copy of that mail on your own computer, so you can delete it from Gmail without losing anything.

Local Eml never deletes anything from your mailbox. It only downloads copies. You decide what to remove in Gmail afterwards, once you've checked the copy is complete.

## Option A: connect over IMAP

This pulls directly from Gmail and can keep fetching new mail later.

1. **Create an app password.** Gmail doesn't accept your normal password from other apps. Turn on 2-Step Verification in your Google account if it isn't on, then visit [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords) and create a password for "Local Eml". Copy the 16 characters.
2. In Local Eml, go to **Import** and choose **IMAP**.
3. Enter host `imap.gmail.com`, port `993`, your Gmail address as the username, and the app password.
4. For everything in the account, set the folder to `[Gmail]/All Mail`. Gmail names this folder in your account's language, for example `[Gmail]/전체보관함` for Korean accounts. Leave `INBOX` to fetch just the inbox.
5. Save it as a profile and run the import. A large account takes a while; you can cancel and continue later, and nothing is downloaded twice.

To keep receiving new mail automatically, turn on background sync for the profile. See [Receiving new mail](../sync).

## Option B: Google Takeout

If you'd rather not create an app password, export an `.mbox` with Takeout and import that. It's a one-time snapshot. See [View a Google Takeout .mbox](./takeout-mbox).

## Check, then delete in Gmail

1. In Local Eml, open **Stats** to see how many messages you have and from when.
2. Spot-check a few old messages, including ones with attachments.
3. In Gmail, search for what you've archived, for example `older_than:5y`, select all, and delete. Then empty the Trash; Gmail keeps deleted mail for 30 days before storage is freed.

Keep the Local Eml library backed up like any other important folder. **Export** writes it to a single `.zip` you can store anywhere.
