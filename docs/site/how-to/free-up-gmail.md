# How to free up Gmail storage without losing mail

Gmail shares its storage with Google Drive and Google Photos, and old email often takes up a lot of it. With Local Eml you can keep a full, searchable copy of that mail on your own computer, then delete it from Gmail.

Local Eml never deletes anything from Gmail. It only downloads copies. You decide what to delete in Gmail yourself, after checking that the copy is complete.

## Option A: connect over IMAP

This downloads straight from Gmail, and can keep fetching new mail later.

1. **Create an app password.** Gmail won't accept your normal password from other apps. Turn on 2-Step Verification in your Google account if it isn't on already. Then go to [myaccount.google.com/apppasswords](https://myaccount.google.com/apppasswords), create a password named "Local Eml", and copy the 16 characters it shows you.
2. In Local Eml, go to **Import** and choose **IMAP**.
3. Enter `imap.gmail.com` as the host, `993` as the port, your Gmail address as the username, and the app password.
4. To get all your mail, set the folder to `[Gmail]/All Mail`. The name depends on your Gmail language; in Korean it's `[Gmail]/전체보관함`. To get only your inbox, leave it as `INBOX`.
5. Save it as a profile and start the import. A large account takes a while. You can stop and continue later, and nothing is downloaded twice.

If you want new mail to keep coming in, turn on background sync for the profile. See [Receiving new mail](../sync).

## Option B: Google Takeout

If you'd rather not create an app password, export your mail with Google Takeout and import the `.mbox` file. This is a one-time copy. See [View a Google Takeout .mbox](./takeout-mbox).

## Check the copy, then delete from Gmail

1. In Local Eml, open [**Stats**](../stats) to see how many messages you have and what dates they cover.
2. Open a few old messages, including some with attachments, to make sure they look right.
3. In Gmail, search for the mail you've copied, for example `older_than:5y` for mail older than five years. Select it all and delete it. Then empty the Trash, because Gmail keeps deleted mail there for 30 days and you won't get the space back until it's gone.

Back up your Local Eml library like any other important folder. **Export** saves it as a single `.zip` you can keep anywhere.
