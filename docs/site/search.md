# Searching and browsing

The **Library** page is where you'll spend most of your time. It lists your mail, with search and filters at the top. Click a message to read it.

![The library](https://hwhang0917.github.io/local-eml/img/en/library.webp)

## Search

The search box looks through the sender, recipients, subject, body and attachment file names of every message. Korean, Japanese and Chinese text work without any setup. When you type several words, a message has to contain all of them, in any field.

### 초성검색 (Korean initial-consonant search)

You can type just the first consonant of each Korean syllable. `ㅎㄱ` finds `한국`, and `ㅇㅁ` finds `이메일`. This works across your whole library.

![Search results](https://hwhang0917.github.io/local-eml/img/en/search.webp)

### Search filters

Add filters to the search box to narrow things down. Say you're looking for a receipt Amazon sent you in 2019 that came with a PDF. Type:

```
receipt from:amazon filename:pdf after:2019 before:2020
```

Click the sliders button at the right of the search box for the full list. Clicking a filter there adds it to your search.

| Filter | Finds mail that… |
|---|---|
| `from:alice` | was sent by someone whose name or address contains `alice` |
| `to:bob@example.com` | was sent to that person, including Cc |
| `cc:bob` | has that person in Cc |
| `subject:invoice` | has the word in its subject |
| `filename:pdf` | has an attachment whose file name contains `pdf` |
| `has:attachment` | has any attachment |
| `is:starred` | you starred |
| `is:unread` / `is:read` | you haven't opened / have opened (needs [read tracking](./settings-cli#settings)) |
| `category:red` | is in that category. Use the colour or the name you gave it; `category:none` finds mail with no category |
| `after:2019-03` | was sent on or after that date |
| `before:2020` | was sent before that date |
| `on:2024-05-17` | was sent on that day, month or year |
| `until:2024-05-31` | was sent on or before that date, including that day |
| `larger:5M` / `smaller:100K` | is bigger / smaller than that size (K, M or G) |
| `"exact phrase"` | contains those words next to each other, in that order |
| `-word` | does **not** contain the word. Works on filters too: `-from:newsletter`, `-has:attachment` |

Dates can be a year (`2019`), a month (`2019-03`) or a day (`2019-03-05`), so `after:2019 before:2020` covers all of 2019. Dates use your computer's time zone, like the date range picker. You can use the filters together with the buttons and the date range picker; everything has to match.

If a filter is misspelled or not finished yet, such as `from:` with nothing after it, it's ignored rather than emptying your results.

## Filters

The buttons next to the search box are shortcuts for search filters. Clicking one writes its filter into the search box, so the box always shows everything you're filtering by, and you can edit it there too.

- **Starred** adds `is:starred`. Click it again to take it out.
- **Unread** adds `is:unread`.
- **Date range** adds `after:` and `until:` with the days you pick.
- **Categories** are a set of colours. Give them names under **Settings → Categories**, and assign one to a message from the list or while reading it. Picking one in the filter adds `category:` with its colour, such as `category:blue`, so the filter keeps working if you rename it.
- **Reset** clears the search box and the sort order.
- **Group by conversation** shows each conversation as one row, using its newest message. Click the count to see the rest.

## Saved searches

If you run the same search often, save it. Type the search, or click the filters you want, then open **Saved** and choose **Save this search**. Give it a name, such as "Receipts" for `receipt filename:pdf -from:newsletter`.

Next time, open **Saved** and click the name. The button shows the name while that search is on. Saving with a name you already used replaces that search, and the × next to a name deletes it. Saved searches are kept with your settings, so they're included in a backup.

## Exporting search results

You can take some of your mail out of Local Eml, for example to send a lawyer every message from your landlord. There are two ways to do it:

- **Export N results** appears next to the filters while you're searching. It exports every message that matches, not just the page you're looking at.
- **Export** appears in the header when you've selected messages. It exports just those. With **Group by conversation** on, a selected row exports the whole conversation.

Each one offers three formats:

| Format | What you get |
|---|---|
| **.eml files in a .zip** | One `.eml` file per message, named by date and subject, plus `index.csv`, a list of them all that opens in Excel or Numbers. |
| **One .mbox file** | All the messages in one file that Thunderbird, Apple Mail or another copy of Local Eml can open. |
| **Print or save as PDF** | All the messages in one printout. A contents page comes first, and each message starts on a new page. Choose **Save as PDF** in the print dialog to get one PDF. You can print up to 200 messages at a time. |

Messages come out oldest first. Unlike the full export on the **Export** page, these files don't include your database, so they hold only the messages you picked. Mail marked as spam or phishing is never included.

## Selecting several messages

Tick the box at the start of a row to select a message, or press `x` on the highlighted one. The box in the header selects everything on the current page. Once something is selected, the header shows what you can do with it:

- **Mark all as read** and **Mark all as unread**, if read tracking is on (see [Settings](./settings-cli#settings)).
- **Delete all**, which asks you to confirm first. The messages and their `.eml` files are removed from your computer, and this can't be undone. If you import from the same source again, they'll come back.

With **Group by conversation** on, selecting a row selects the whole conversation, and the action applies to every message in it.

## Reading a message

HTML email is shown in a locked-down frame so scripts in the email can't run. Images hosted on the internet are blocked until you click to load them, which stops senders from seeing that you opened the message. Before a link opens, Local Eml shows you where it really goes.

Images, PDFs, audio, video, text, Hangul (HWP, HWPX), Word (DOCX) and Excel (XLSX, XLS) attachments can be previewed. Other attachments are downloaded when you click them.

## Speed

Browsing and searching stay instant even with more than 100,000 messages. The one exception is **Group by conversation**, which can take about a second per page on a very large library. If that bothers you, turn grouping off.

| Action | 10,000 messages | 100,000 messages |
|---|---|---|
| Browse / change page | under 1 ms | under 1 ms |
| Search for a typical word | under 1 ms | about 2 ms |
| 초성 search | about 5 ms | about 50 ms |
| Search with filters such as `from:` and `has:attachment` | under 1 ms | about 10 ms |
| Grouped by conversation | about 85 ms | about 0.9 s |

Matching a word against attachment names adds a few milliseconds on a very large library. These were measured on an ordinary laptop with generated test mail, timing one page of 50 messages including the total count.
