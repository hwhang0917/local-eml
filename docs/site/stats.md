# Library stats

The **Stats** page gives you an overview of what's in your library: how much mail you have, who it's from, when it arrives and what takes up the space. It helps when you're deciding what to keep, or checking that a big import brought in what you expected.

![The stats page](https://hwhang0917.github.io/local-eml/img/en/stats.webp)

Most things on the page are links. Click a sender, a domain, a category or an inbox health number to open the library with that filter already applied.

## What's on the page

- **Totals** at the top: the number of emails, their total size, how many are starred and how many have attachments.
- **Mail over time** shows how many emails were sent each month. Switch to **Yearly** to see the bigger picture. Hover over the chart for exact numbers.
- **When mail arrives** is a grid of weekdays and hours, in your own time zone. The darker the square, the more mail was sent at that time, so you can see at a glance whether your mail is mostly work hours or late evenings. The busiest slot is named underneath.
- **Top senders** lists the people who've sent you the most. Switch to **Domains** to group them by company or provider instead, such as everyone at `example.com`.
- **Inbox health** counts unread, starred and uncategorized mail, plus messages you've marked as spam or phishing and ones with a phishing warning. Unread only appears when [read tracking](./settings-cli#settings) is on.
- **What takes up space** splits the library's size into message text and attachments by type: PDFs, images, audio and video, and so on. Click a type to open the [Attachments](./attachments) page with the largest files of that type first.
- **Largest emails** lists the five biggest messages. Click one to open it.
- **By category** shows how many messages are in each colour category.

## App resources

At the bottom, **App resources** shows how Local Eml itself is doing, updated every 3 seconds while the page is open: how much memory and CPU it uses, how big the library is on disk (email files, the database and search index, and logs), and how much space is left on that drive. The last line shows how long it has been running and which version you have. This is handy if you leave the background service running and want to check it isn't using more than it should.

Emails without a date can't be placed on the chart or the grid. If you have any, a note at the top tells you how many.
