# How to view a Google Takeout .mbox file

Google Takeout exports your Gmail as one or more `.mbox` files. Most mail apps can't open these directly. Local Eml imports them and turns each message into a searchable `.eml` file.

## 1. Export from Google Takeout

1. Go to [takeout.google.com](https://takeout.google.com).
2. Click **Deselect all**, then tick **Mail**. If you only want some labels, click **All Mail data included** and pick them.
3. Choose how you want to receive the export and the file size, then click **Create export**. Google emails you when it's ready. For a large mailbox this can take hours or even days.
4. Download the archive and unzip it. Inside, a `Mail` folder holds one or more `.mbox` files, usually named after your labels, such as `All mail Including Spam and Trash.mbox`.

## 2. Import into Local Eml

1. Open Local Eml and go to **Import**.
2. Drag the `.mbox` file onto the page. Make sure you unzip the archive first, because Local Eml can't read an `.mbox` that's still inside Takeout's zip.
3. Wait for it to finish. If Takeout gave you files that overlap, each message is still imported only once. You can cancel at any time.

A large export takes a while to import, but once it's done, browsing and searching are fast, even with more than 100,000 messages.

## 3. Check and search

Open the **Library**. Messages keep their original dates, senders and attachments. Use the date range filter to find a period, or search by sender, subject or text.

## Getting new mail too

A Takeout export is a one-time copy. If you want new Gmail messages to keep arriving, connect Gmail over IMAP as well; [Free up Gmail storage](./free-up-gmail) shows how. You can use both. Messages that are already in your library are skipped.
