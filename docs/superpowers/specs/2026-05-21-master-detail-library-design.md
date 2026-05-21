# Master-Detail Library with Pop-out Detail Windows — Design

Date: 2026-05-21

## Goal

Replace the full-page navigation between Library and Viewer with a two-pane
master-detail layout, and let the user pop a detail out into its own
chrome-less browser window. As a consequence of not navigating away, the
library's search / tag filter / sort / pagination — and the current selection —
persist automatically; they are further made reload- and link-safe by living in
the URL query string.

This is a purely frontend restructuring. The Go backend, REST/SSE API,
sanitizer, and tag endpoints are untouched.

## Background

Today:

- `LibraryPage.vue` (`/`) holds `q`, `tag`, `sort`, `order`, `offset` in local
  refs. Clicking a row calls `router.push({ name: 'viewer', params: { sha } })`,
  which unmounts the page; on return the refs reset to defaults.
- `ViewerPage.vue` (`/email/:sha`) is one large component that loads the email +
  parts, renders the header card, a tag editor, and HTML/Text/Raw/Attachments
  tabs with a remote-image toggle. Its only way back is a `RouterLink` to `/`.

The viewer's body is already self-contained around a single `sha`, which makes
it straightforward to extract into a component reused in two places.

## Architecture

### Component decomposition

- **`web/src/components/EmailDetail.vue`** (new) — prop `sha: string`. Owns
  everything currently inside `ViewerPage`'s body: data loading
  (`getEmail` + `getParts`, the `tab`/`textBody`/`rawBody`/`showRemote` state and
  watchers), the header card, the `TagsInput` editor with `onTagAdd`/`onTagRemove`,
  the four tabs, the remote-image toggle, and a new **pop-out** button. It renders
  no navigation chrome and no "back" link. It reloads when `sha` changes
  (`watch(() => props.sha, load, { immediate: true })`).
- **`web/src/pages/LibraryPage.vue`** — becomes the two-pane shell: the existing
  collapsible tags sidebar, the email-list table, and `<EmailDetail :sha="selected">`
  in a right-hand pane. When no email is selected, the right pane shows a muted
  placeholder (`library.select_prompt`). Layout target:

  ```
  ┌──────┬─────────────────┬──────────────────────────┐
  │ Tags │  Email list     │  Detail pane              │
  │      │ ▸ Subject A     │  Subject A                │
  │ All  │ ▸ Subject B ◀── │  From / To / Date         │
  │ work │ ▸ Subject C     │  [HTML][Text][Raw][Att]   │
  │      │ ◀ prev  next ▶  │  body... [⧉ pop out]      │
  └──────┴─────────────────┴──────────────────────────┘
  ```

- **`web/src/pages/ViewerPage.vue`** — slims to a thin wrapper that reads its
  `sha` prop and renders `<EmailDetail :sha="sha" />`. This route is the
  chrome-less pop-out target.

### Routing & the chrome-less window

- `/` (library) carries the in-pane detail; selection lives in `?sel=<sha>`.
- `/email/:sha` is unchanged in path and `props: true`, and gains
  `meta: { chromeless: true }`.
- **`App.vue`** branches on `route.meta.chromeless`:
  - chromeless → render only `<RouterView />` inside a tight container, no top
    nav header, no `max-w-[1800px]` centering needed (reading width comes from
    the window size).
  - otherwise → the current header + `<main>` layout, unchanged.
- **Pop-out** (button inside `EmailDetail`): `window.open('/email/' + sha,
  '_blank', 'popup,width=820,height=900')`. Each invocation may open another
  window; the library pane keeps its current selection. The chrome-less window
  sets `document.title` to the email subject (falling back to the existing
  app-title behaviour when subject is empty) so multiple windows are
  distinguishable in the OS window list.

### State in the URL (single source of truth)

All library state moves into `route.query`: `q`, `tag`, `sort`, `order`,
`offset`, `sel`.

- A watcher on `route.query` is the **only** trigger for `load()` and for
  computing the selected sha. The component derives its working values from the
  query (with defaults: `sort=sent_at`, `order=desc`, `offset=0`, the rest
  empty) rather than holding independent source-of-truth refs.
- UI actions update the URL:
  - search box → **debounced** `router.push` (≈250ms; one history entry per
    search, not per keystroke),
  - tag click / sort / pagination → `router.push`,
  - selection (`sel`) → `router.replace` — clicking through emails does not
    flood back-history, so back/forward steps through *filter* states.
- Result: reload, copy-paste link, and back/forward all behave correctly. The
  pop-out target reuses whatever `sel`/`:sha` resolves to.

Implementation note: to avoid feedback loops, the query→state watcher only reads;
state→query happens in explicit handlers. `offset` resets to 0 when `q` or `tag`
changes (current behaviour), expressed as part of those push handlers.

### Removed / unchanged

- The "← back to library" `RouterLink` is removed (in-pane: the list is adjacent;
  pop-out: the user closes the window).
- `lib/api.ts`, sanitizer, SSE/Hub, tag endpoints — untouched.
- New i18n keys, en/ko parity: `viewer.pop_out`, `library.select_prompt`.

## Testing

Per project convention the maintainer runs `make build` / `serve` and verifies
in the browser; this spec lists what to check rather than running it.

Manual verification:

- Click a row → detail loads in the right pane, no navigation, list state intact.
- Search/tag/sort/page → URL query updates; reload restores the same view and
  selection; browser back steps through filter changes.
- Copy the URL into a new tab → same filtered list + selected email.
- Pop-out → a chrome-less window opens showing only the detail, titled with the
  subject; the library keeps its selection; a second pop-out opens another window.
- Tabs, remote-image toggle, tag add/remove, attachment download all work in both
  the in-pane detail and the popped-out window.

`gofmt`/`go vet`/`go test ./...` remain green (no Go changes); `npm run build`
type-checks the SPA.

## Out of scope (v1)

- Keyboard list navigation (j/k or arrow keys to move selection).
- A draggable splitter to resize the list/detail panes (fixed proportions in v1).
- Live propagation of tag edits made in a pop-out window back into the main
  window's list (a manual refresh / reselect covers it).
- Mobile / very-narrow responsive collapsing of the three columns (desktop
  single-user tool; the tags sidebar is already collapsible).
