# Resizable List│Detail Split + Toggle-off Selection Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Library's list│detail divider draggable (persisted) and make clicking the already-selected row clear the selection.

**Architecture:** Both changes are confined to `web/src/pages/LibraryPage.vue`. The list `<section>` and detail `<aside>` are wrapped in reka-ui's `SplitterGroup`/`SplitterPanel`/`SplitterResizeHandle` (already a dependency) with `autoSaveId` localStorage persistence; the tags sidebar stays outside, unchanged. The `select()` handler becomes a toggle.

**Tech Stack:** Vue 3 `<script setup lang=ts>`, reka-ui@2.9.7 Splitter primitives, Tailwind v4, vue-router query state.

**Verification note:** No frontend test framework exists; per project convention the maintainer runs build/serve. The per-task gate is `npm run type-check` (`vue-tsc --noEmit`, run from `web/`). Manual browser checks are listed at the end. Do NOT add a test framework.

---

## File Structure

- `web/src/pages/LibraryPage.vue` — the only file changed. Task 1 edits the `select()` function in `<script setup>`. Task 2 adds three reka-ui imports and restructures the `<template>` to wrap the list/detail in a splitter. The trailing `<script lang="ts">` `Th` helper block is untouched in both tasks.

reka-ui's Splitter components are already installed (`reka-ui@2.9.7`); no `npm install` is needed.

---

## Task 1: Toggle-off selection

**Files:**
- Modify: `web/src/pages/LibraryPage.vue` (the `select` function, currently lines 123-126)

- [ ] **Step 1: Replace the `select` function**

The current function short-circuits when re-clicking the selected row:

```ts
function select(sha: string) {
  if (sha === selected.value) return
  replaceQuery({ sel: sha })
}
```

Replace it with a toggle — clicking the selected row clears the selection (passing `undefined` makes `mergeQuery` drop the `sel` key):

```ts
function select(sha: string) {
  replaceQuery({ sel: sha === selected.value ? undefined : sha })
}
```

- [ ] **Step 2: Type-check**

Run (from `web/`): `npm run type-check`
Expected: PASS, no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/LibraryPage.vue
git commit -m "feat(web): toggle off selection when re-clicking the selected row

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: Resizable list│detail split

**Files:**
- Modify: `web/src/pages/LibraryPage.vue` (add imports in `<script setup>`; replace the `<template>` block)

reka-ui prop reference (verified against `reka-ui@2.9.7` typings):
- `SplitterGroup`: `direction` ('horizontal' | 'vertical', required), `autoSaveId` (string — persists the layout to localStorage).
- `SplitterPanel`: `defaultSize` (number, percent), `minSize` (number, percent).
- `SplitterResizeHandle`: renders a focusable separator (arrow-key resizable, ARIA set automatically); accepts `class`.

- [ ] **Step 1: Add the reka-ui Splitter imports**

In `web/src/pages/LibraryPage.vue`, immediately after the existing `import EmailDetail from '@/components/EmailDetail.vue'` line, add:

```ts
import { SplitterGroup, SplitterPanel, SplitterResizeHandle } from 'reka-ui'
```

- [ ] **Step 2: Replace the entire `<template>` block**

Replace the whole `<template>...</template>` (currently lines 135-239) with the following. The only structural change vs. the current template is: the list `<section>` and detail `<aside>` are now wrapped in `SplitterGroup` → two `SplitterPanel`s with a `SplitterResizeHandle` between them; the `<section>` loses `flex-1` (the panel controls width) and the `<aside>` loses `w-[40rem] shrink-0` (likewise). Everything inside the section and aside is unchanged.

```vue
<template>
  <div class="flex gap-6">
    <Sidebar v-if="sidebarOpen">
      <SidebarHeader>
        <SidebarTitle>{{ t('library.tags') }}</SidebarTitle>
        <button
          @click="sidebarOpen = false"
          :title="t('library.collapse')"
          class="h-7 w-7 rounded-sm text-muted-foreground hover:bg-accent hover:text-foreground flex items-center justify-center"
        >
          <ChevronsLeft class="h-4 w-4" />
        </button>
      </SidebarHeader>
      <SidebarContent>
        <SidebarMenu>
          <SidebarMenuItem>
            <SidebarMenuButton :active="tag === ''" @click="setTag('')">
              {{ t('library.all') }}
              <span class="ml-auto text-xs text-muted-foreground">{{ total }}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
          <SidebarMenuItem v-for="t2 in tags" :key="t2.name">
            <SidebarMenuButton :active="tag === t2.name" @click="setTag(t2.name)">
              {{ t2.name }}
              <span class="ml-auto text-xs text-muted-foreground">{{ t2.count }}</span>
            </SidebarMenuButton>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarContent>
    </Sidebar>

    <button
      v-else
      @click="sidebarOpen = true"
      :title="t('library.expand')"
      class="self-start h-10 w-10 rounded-sm border border-hairline bg-card text-muted-foreground hover:bg-accent hover:text-foreground flex items-center justify-center"
    >
      <ChevronsRight class="h-4 w-4" />
    </button>

    <SplitterGroup direction="horizontal" auto-save-id="library-split" class="flex-1 min-w-0">
      <SplitterPanel :default-size="58" :min-size="30" class="min-w-0">
        <section class="min-w-0">
          <div class="flex items-center gap-3 mb-4">
            <Input v-model="searchInput" :placeholder="t('library.search_placeholder')" class="max-w-md" />
            <span class="text-sm text-muted-foreground ml-auto">{{ pageInfo }}</span>
          </div>

          <Card class="overflow-hidden">
            <table class="w-full text-sm">
              <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
                <tr>
                  <th class="text-left px-3 py-2 w-10"></th>
                  <Th :label="t('library.col.date')" col="sent_at" :sort="sort" :order="order" @sort="setSort" />
                  <Th :label="t('library.col.from')" col="from_addr" :sort="sort" :order="order" @sort="setSort" />
                  <Th :label="t('library.col.subject')" col="subject" :sort="sort" :order="order" @sort="setSort" />
                  <Th :label="t('library.col.size')" col="size_bytes" :sort="sort" :order="order" @sort="setSort" align="right" />
                </tr>
              </thead>
              <tbody>
                <tr v-if="loading && items.length === 0">
                  <td colspan="5" class="px-3 py-6 text-center text-muted-foreground">{{ t('library.loading') }}</td>
                </tr>
                <tr v-else-if="items.length === 0">
                  <td colspan="5" class="px-3 py-6 text-center text-muted-foreground">
                    {{ t('library.no_emails') }}
                    <RouterLink to="/import" class="underline">{{ t('library.import_some') }}</RouterLink>
                  </td>
                </tr>
                <tr
                  v-for="e in items"
                  :key="e.sha256"
                  class="border-t hover:bg-accent/50 cursor-pointer"
                  :class="{ 'bg-accent': e.sha256 === selected }"
                  @click="select(e.sha256)"
                >
                  <td class="px-3 py-2 text-muted-foreground">
                    <span v-if="e.has_attachments" :title="t('library.has_attachments')">📎</span>
                  </td>
                  <td class="px-3 py-2 whitespace-nowrap text-muted-foreground">{{ formatDate(e.sent_at) }}</td>
                  <td class="px-3 py-2 truncate max-w-[18rem]" :title="e.from">{{ e.from }}</td>
                  <td class="px-3 py-2 truncate">
                    {{ e.subject || t('library.no_subject') }}
                    <span v-for="t2 in e.tags" :key="t2" class="ml-1 inline-block text-[10px] px-1.5 py-0.5 rounded bg-accent text-accent-foreground">{{ t2 }}</span>
                    <span class="ml-2 text-xs text-muted-foreground">{{ shortSHA(e.sha256) }}</span>
                  </td>
                  <td class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(e.size_bytes) }}</td>
                </tr>
              </tbody>
            </table>
          </Card>

          <div class="flex justify-between items-center mt-4">
            <Button variant="outline" size="sm" :disabled="offset === 0" @click="setOffset(Math.max(0, offset - limit))">{{ t('library.prev') }}</Button>
            <span class="text-sm text-muted-foreground">{{ pageInfo }}</span>
            <Button variant="outline" size="sm" :disabled="offset + limit >= total" @click="setOffset(offset + limit)">{{ t('library.next') }}</Button>
          </div>
        </section>
      </SplitterPanel>

      <SplitterResizeHandle
        class="w-1.5 mx-1 shrink-0 rounded-full bg-muted hover:bg-accent focus-visible:bg-accent focus:outline-none cursor-col-resize transition-colors"
      />

      <SplitterPanel :default-size="42" :min-size="25" class="min-w-0">
        <aside class="min-w-0">
          <EmailDetail v-if="selected" :sha="selected" />
          <Card v-else class="p-10 text-center text-muted-foreground">
            {{ t('library.select_prompt') }}
          </Card>
        </aside>
      </SplitterPanel>
    </SplitterGroup>
  </div>
</template>
```

- [ ] **Step 3: Type-check**

Run (from `web/`): `npm run type-check`
Expected: PASS, no errors. (If reka-ui reports a prop type error, the prop names above were verified against the installed typings — investigate and report rather than guessing.)

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/LibraryPage.vue
git commit -m "feat(web): draggable, persisted list/detail split (reka-ui Splitter)

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Manual Verification (maintainer)

Run `make web-dev` (Vite dev server) or `make build && ./local-eml serve`, then:

1. Drag the divider between the list and the detail pane → both panes resize; neither shrinks past its minimum (list 30%, detail 25%).
2. Reload the page → the chosen split ratio is restored (localStorage key `react-resizable-panels:library-split` or similar set by reka-ui's `autoSaveId`).
3. Focus the divider (Tab) and press ← / → → the split adjusts via keyboard.
4. Click an unselected row → it selects, detail loads, row highlights. Click the same row again → selection clears, detail pane shows "Select an email to read.", highlight clears, and `?sel` drops from the URL. Click a different row → selection moves to it.
5. The tags sidebar collapse/expand toggle still works; collapsing/expanding it does not break the split.
6. Open a row, click ⧉ pop out → the standalone `/email/:sha` window still renders detail-only with no splitter.

Backend untouched; `go test ./...` remains green.

---

## Self-Review

- **Spec coverage:** "Change 1 — Resizable split" → Task 2 (SplitterGroup/Panel/Handle, `auto-save-id="library-split"`, min-sizes 30/25, default 58/42, handle styling, sidebar stays outside, `flex-1` removed from section, `w-[40rem] shrink-0` removed from aside). "Change 2 — Toggle-off selection" → Task 1 (`select` toggles `sel`). Out-of-scope items (sidebar drag, vertical split, pop-out window) are correctly not implemented.
- **Placeholder scan:** none — both tasks contain the exact code to write.
- **Type consistency:** import names `SplitterGroup`/`SplitterPanel`/`SplitterResizeHandle` match the template usage; props `direction`/`auto-save-id`/`:default-size`/`:min-size` match reka-ui@2.9.7 typings; `select`/`selected`/`replaceQuery`/`mergeQuery` are pre-existing and unchanged in signature.
