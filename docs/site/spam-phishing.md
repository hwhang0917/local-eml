# Spam and phishing

Local Eml warns you about messages that look like phishing, and lets you mark messages as spam or phishing yourself. These are separate features.

## Phishing warnings

Every message is checked on your computer for common signs of phishing. Nothing is sent anywhere, and it doesn't rely on lists of suspicious words. It looks for:

- A link whose text shows one website but actually goes to another
- A link to a bare IP address instead of a website name
- A website name made to look like another one, or one that uses unusual characters
- A message that fails the sender checks mail servers use (SPF, DKIM or DMARC)
- Replies that would go to a different website than the sender's
- A sender name that contains an email address from a different website
- An attached program or script
- A form or password box inside the message

If a message has any of these, you'll see a yellow banner when you open it and a warning icon in the list. It's only a warning. Nothing is hidden automatically.

If you know a message is fine, choose **Not spam** on the banner or from the flag menu. The warning goes away for that message only. To turn warnings off completely, go to **Settings → Spam & phishing**.

## Marking a message as spam or phishing

Use the shield icon while reading a message to mark it as **Spam** or **Phishing**. Marked messages are:

- Hidden from the library and from conversations.
- Shown as plain text only. Formatting, images, attachments and links are all turned off. Local Eml refuses to load those parts, so they stay off even if something tries to request them.

**Settings → Spam & phishing** lists everything you've marked, and you can unmark messages there.
