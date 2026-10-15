# How to view a Google Takeout .mbox file

Google Takeout exports Gmail as one or more `.mbox` files, a format most mail programs can't open directly. Local Eml imports them and turns each message into a searchable `.eml`.

## 1. Export from Google Takeout

1. Go to [takeout.google.com](https://takeout.google.com).
2. **Deselect all**, then tick **Mail**. Use **All Mail data included** to pick specific labels if you don't want everything.
3. Choose a delivery method and file size, then **Create export**. Google emails you when it's ready; large mailboxes can take hours or days.
4. Download the archive and extract it. Inside you'll find a `Mail` folder with one or more `.mbox` files, typically named after your labels, for example `All mail Including Spam and Trash.mbox`.

## 2. Import into Local Eml

1. Open Local Eml and go to **Import**.
2. Drag the `.mbox` file onto the page. Extract the archive first; Local Eml reads `.mbox` files directly, not from inside Takeout's zip.
3. Watch the progress. Duplicates are detected by content, so if Takeout gave you overlapping files, each message still lands once. You can cancel at any time.

Large exports take a while to import, but browsing and search stay fast afterwards, even past 100,000 messages.

## 3. Check and search

Open the **Library**. Messages keep their original dates, senders and attachments. Use the date range filter to find a period, or search by sender, subject or body.

## Keeping it in sync

Takeout is a one-time snapshot. If you want new Gmail messages to keep arriving, add Gmail as an IMAP source instead; see [Free up Gmail storage](./free-up-gmail). Both can coexist: duplicates are skipped.
