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
| `after:2019-03` | was sent on or after that date |
| `before:2020` | was sent before that date |
| `on:2024-05-17` | was sent on that day, month or year |
| `larger:5M` / `smaller:100K` | is bigger / smaller than that size (K, M or G) |
| `"exact phrase"` | contains those words next to each other, in that order |
| `-word` | does **not** contain the word. Works on filters too: `-from:newsletter`, `-has:attachment` |

Dates can be a year (`2019`), a month (`2019-03`) or a day (`2019-03-05`), so `after:2019 before:2020` covers all of 2019. Dates use your computer's time zone, like the date range picker. You can use the filters together with the buttons and the date range picker; everything has to match.

If a filter is misspelled or not finished yet, such as `from:` with nothing after it, it's ignored rather than emptying your results.

## Filters

- **Starred** shows only the messages you've starred.
- **Categories** are a set of colours. Give them names under **Settings → Categories**, assign one to a message from the list or while reading it, and then filter by it.
- **Date range** shows only messages sent between two dates.
- **Group by conversation** shows each conversation as one row, using its newest message. Click the count to see the rest.

## Selecting several messages

Tick the box at the start of a row to select a message, or press `x` on the highlighted one. The box in the header selects everything on the current page. Once something is selected, the header shows what you can do with it:

- **Mark all as read** and **Mark all as unread**, if read tracking is on (see [Settings](./settings-cli#settings)).
- **Delete all**, which asks you to confirm first. The messages and their `.eml` files are removed from your computer, and this can't be undone. If you import from the same source again, they'll come back.

With **Group by conversation** on, selecting a row selects the whole conversation, and the action applies to every message in it.

## Reading a message

HTML email is shown in a locked-down frame so scripts in the email can't run. Images hosted on the internet are blocked until you click to load them, which stops senders from seeing that you opened the message. Before a link opens, Local Eml shows you where it really goes.

Images, PDFs, audio, video and text attachments can be previewed. Other attachments are downloaded when you click them.

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
