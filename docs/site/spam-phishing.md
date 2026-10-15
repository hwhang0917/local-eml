# Spam and phishing

Local Eml has two separate tools: a warning that's computed, and a flag that you set.

## Phishing warnings

Every message is checked offline for structural signs of phishing. There are no keyword lists and nothing is sent anywhere. The signals are:

- Link text names one site while the link goes to another.
- A link to a bare IP address.
- A lookalike or non-ASCII domain.
- Failed SPF, DKIM or DMARC in the message headers.
- A Reply-To on a different site than the sender.
- A display name that embeds an address on another site.
- An executable or script attachment.
- A form or password field in the body.

A message with any of these shows an amber banner in the viewer and a warning icon in the list. It's a warning, never a verdict: nothing is hidden automatically.

If you know a message is fine, choose **Not spam** from the banner or the flag menu. The warning disappears for that message only. Turn warnings off entirely under **Settings → Spam & phishing**.

## Flagging a message

The shield icon in the viewer flags a message as **Spam** or **Phishing**. Flagged mail is:

- Hidden from the library and from conversation views.
- Shown as plain text only when opened: no HTML, no images, no attachments, no clickable links. The server refuses to serve those parts, so this holds even if a page asks for them.

**Settings → Spam & phishing** lists everything you've flagged, and a button there unflags a message.
