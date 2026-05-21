# Resizable List│Detail Split + Toggle-off Selection — Design

Date: 2026-05-21

## Goal

Two refinements to the two-pane master-detail Library (see
`2026-05-21-master-detail-library-design.md`):

1. Make the divider between the email list and the detail pane draggable, so the
   user can rebalance the two panes; persist the chosen ratio across reloads.
2. Clicking the row of the already-selected email clears the selection (toggle
   off), returning the detail pane to its placeholder.

Both changes are confined to `web/src/pages/LibraryPage.vue`.

## Background

`LibraryPage.vue` lays out `flex gap-6`: a collapsible tags `Sidebar` (toggled,
persisted via `useStorage('library-sidebar-open')`), then the email-list
`<section class="flex-1 min-w-0">`, then a fixed-width detail
`<aside class="w-[40rem] shrink-0 min-w-0">` rendering `<EmailDetail :sha="selected">`
or the `library.select_prompt` placeholder. Selection lives in `route.query.sel`;
`select(sha)` currently calls `replaceQuery({ sel: sha })` and short-circuits when
`sha === selected.value` (a no-op guard added during master-detail review).

`reka-ui@2.9.7` (already a dependency) provides `SplitterGroup`, `SplitterPanel`,
and `SplitterResizeHandle`, including `autoSaveId` localStorage persistence and
keyboard/ARIA support on the handle.

## Change 1 — Resizable split

Wrap the list `<section>` and the detail `<aside>` in a horizontal splitter. The
tags `Sidebar` and its collapsed-state toggle button remain **outside** the
splitter group, unchanged.

```
flex gap-6
├─ Sidebar (or collapsed toggle button)   ← unchanged, outside the group
└─ SplitterGroup direction="horizontal" autoSaveId="library-split"  (flex-1)
   ├─ SplitterPanel :default-size="58" :min-size="30"   → list <section>
   ├─ SplitterResizeHandle                              → draggable bar
   └─ SplitterPanel :default-size="42" :min-size="25"   → detail <aside>
```

- Sizes are percentages of the group width. `autoSaveId="library-split"` persists
  the ratio to localStorage automatically — consistent with the sidebar's
  persistence approach.
- The list `<section>` keeps `min-w-0`; its `flex-1` is no longer needed (the
  panel controls width). The detail `<aside>` drops `w-[40rem] shrink-0`; the
  panel controls its width. Both retain `min-w-0` so inner content can truncate.
- The `gap-6` between list and detail goes away (the panels are adjacent); the
  `SplitterResizeHandle` provides the visual separation. The `gap-6` between the
  sidebar and the splitter group is preserved.
- Handle styling: a thin vertical bar (`w-1.5`), `cursor-col-resize`,
  `bg-transparent hover:bg-accent` (or a muted hairline that brightens on
  hover/drag). reka-ui marks the handle with the appropriate ARIA role and makes
  it focusable + arrow-key resizable without extra code.
- The detail panel always renders; the existing `v-if="selected"` /
  placeholder logic is unchanged inside it.

Imports added to the `<script setup>`: `SplitterGroup`, `SplitterPanel`,
`SplitterResizeHandle` from `reka-ui`.

## Change 2 — Toggle-off selection

Replace the no-op guard in `select` with a toggle:

```ts
function select(sha: string) {
  replaceQuery({ sel: sha === selected.value ? undefined : sha })
}
```

`mergeQuery` already drops keys whose value is `undefined`, so passing
`{ sel: undefined }` removes `?sel` from the URL. Clearing the selection makes
`selected` an empty string, so the detail pane falls back to the
`library.select_prompt` placeholder and the row highlight
(`:class="{ 'bg-accent': e.sha256 === selected }"`) turns off. Selecting a
different row behaves as before. This still uses `replaceQuery` (no history
flooding from selection changes).

## Out of scope

- Making the tags sidebar width draggable (it stays a collapse/expand toggle).
- A vertical (stacked) split or per-route split ratios.
- Changing the standalone pop-out window (`/email/:sha`), which has no split.

## Testing

No frontend test framework exists; per project convention the maintainer runs
build/serve. Gate: `npm run type-check` (vue-tsc) passes. Manual checks:

- Drag the handle → list and detail resize; the ratio is restored after reload
  (localStorage `library-split`); neither pane shrinks past its min-size.
- Keyboard: focus the handle, arrow keys adjust the split.
- Click an unselected row → it selects and the detail loads. Click the same row
  again → selection clears, detail pane shows the placeholder, highlight clears,
  `?sel` drops from the URL. Click another row → selection moves.
- The tags sidebar collapse/expand toggle still works and is unaffected.
- Pop-out window (`/email/:sha`) still renders detail-only with no splitter.

Backend untouched; `go test ./...` remains green.
