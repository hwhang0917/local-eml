# Searching and browsing

The library page is the home screen: a list of your mail with search, filters and a reading pane one click away.

![The library](https://hwhang0917.github.io/local-eml/img/en/library.webp)

## Full-text search

The search box matches sender, subject and body at once. Korean and other CJK languages work out of the box; the index is built with SQLite FTS5.

### 초성검색

Type only the initial consonants of Hangul syllables to match any word that starts with them. `ㅎㄱ` finds `한국`, `ㅇㅁ` finds `이메일`. It works across the whole library.

![Search results](https://hwhang0917.github.io/local-eml/img/en/search.webp)

## Filters

- **Starred** shows only messages you starred.
- **Categories** are a fixed set of colours you can name under **Settings → Categories**. Assign one from the list or the viewer, then filter by it.
- **Date range** limits the list to messages sent between two dates.
- **Group by conversation** collapses each thread to its newest message, with a count you can expand.

## Reading

HTML messages render inside a sandboxed frame. Remote images are blocked by default and shown as a placeholder; one click loads them for that message. Links open in a new tab after a confirmation that shows the real destination.

Attachments are listed with a preview for images, PDFs, audio, video and text. Other types download.

## Performance

Browsing and search stay effectively instant well past 100,000 messages. The one view that grows with library size is **group by conversation**: on a very large library it can take about a second per page. If that feels slow, turn the grouping toggle off.

| Operation | 10k emails | 100k emails |
|---|---|---|
| Browse / paginate | < 1 ms | < 1 ms |
| Search, typical term | < 1 ms | ~2 ms |
| 초성 search | ~5 ms | ~50 ms |
| Conversation-grouped list | ~85 ms | ~0.9 s |

Measured on a mid-range laptop with synthetic libraries, timing one 50-row page including its total count.
