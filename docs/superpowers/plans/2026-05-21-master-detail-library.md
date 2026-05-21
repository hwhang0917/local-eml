# Master-Detail Library with Pop-out Detail Windows — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the Library into a two-pane master-detail view whose filter/sort/pagination/selection live in the URL, and let a selected email pop out into a chrome-less standalone window.

**Architecture:** Extract the viewer body into a reusable `EmailDetail.vue` used both in the library's right pane and in the slimmed-down `/email/:sha` route. `App.vue` renders that route without nav chrome via a `chromeless` route-meta flag. `LibraryPage.vue` derives all of its state from `route.query` (`q`, `tag`, `sort`, `order`, `offset`, `sel`) and writes it back with `push` (filters) / `replace` (selection).

**Tech Stack:** Vue 3 `<script setup lang="ts">`, vue-router, vue-i18n, @vueuse/core (`useDebounceFn`, `useStorage`), Tailwind v4.

**Verification note:** This project has no frontend test framework (no vitest), and per the maintainer's convention the maintainer runs build/serve. The per-task gate is `npm run type-check` (which runs `vue-tsc --noEmit`); each task also lists what to verify manually in the browser. Do **not** add a test framework.

---

## File Structure

- `web/src/locales/en.json`, `web/src/locales/ko.json` — add `viewer.pop_out`, `library.select_prompt`; remove the now-unused `viewer.back`.
- `web/src/components/EmailDetail.vue` — **new.** Reusable detail: data load, header card, tag editor, HTML/Text/Raw/Attachments tabs, remote-image toggle, pop-out button. Prop `sha: string`, prop `standalone?: boolean`.
- `web/src/pages/ViewerPage.vue` — slimmed to a thin wrapper rendering `<EmailDetail :sha standalone />`.
- `web/src/router.ts` — add `meta.chromeless` to the `viewer` route.
- `web/src/App.vue` — branch layout on `route.meta.chromeless`; let the chrome-less route own `document.title`.
- `web/src/pages/LibraryPage.vue` — two-pane shell; URL-query-driven state; `<EmailDetail>` in the right pane.

All commands below run from `web/` unless noted.

---

## Task 1: i18n keys

**Files:**
- Modify: `web/src/locales/en.json`
- Modify: `web/src/locales/ko.json`

- [ ] **Step 1: Add the `pop_out` key and remove `back` in `en.json`**

In the `"viewer"` object, replace the `"back"` line with a `"pop_out"` entry. Change:

```json
    "back": "← back to library",
    "from": "From",
```

to:

```json
    "pop_out": "⧉ Open in new window",
    "from": "From",
```

- [ ] **Step 2: Add `select_prompt` in `en.json`**

In the `"library"` object, add after the `"all"` line:

```json
    "all": "All",
    "select_prompt": "Select an email to read.",
```

- [ ] **Step 3: Mirror both changes in `ko.json`**

In `"viewer"`, replace:

```json
    "back": "← 라이브러리로",
    "from": "보낸 사람",
```

with:

```json
    "pop_out": "⧉ 새 창에서 열기",
    "from": "보낸 사람",
```

In `"library"`, add after the `"all"` line:

```json
    "all": "전체",
    "select_prompt": "읽을 이메일을 선택하세요.",
```

- [ ] **Step 4: Verify JSON is valid**

Run: `jq empty src/locales/en.json && jq empty src/locales/ko.json && echo OK`
Expected: `OK` (no parse error).

- [ ] **Step 5: Commit**

```bash
git add web/src/locales/en.json web/src/locales/ko.json
git commit -m "i18n: pop-out + select-prompt keys; drop viewer.back"
```

---

## Task 2: Extract `EmailDetail.vue`

**Files:**
- Create: `web/src/components/EmailDetail.vue`

This component is the current `ViewerPage.vue` body, minus the back link, plus a pop-out button and standalone-window title handling. It takes `sha` and an optional `standalone` flag.

- [ ] **Step 1: Create `web/src/components/EmailDetail.vue`**

```vue
<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import type { AcceptableInputValue } from 'reka-ui'
import { api, type Email, type PartsManifest } from '@/lib/api'
import { formatBytes, formatDate } from '@/lib/format'
import Card from '@/components/ui/Card.vue'
import {
  TagsInput,
  TagsInputInput,
  TagsInputItem,
  TagsInputItemText,
  TagsInputItemDelete,
} from '@/components/ui/tags-input'

const props = defineProps<{ sha: string; standalone?: boolean }>()
const { t } = useI18n()

const email = ref<Email | null>(null)
const parts = ref<PartsManifest | null>(null)
const tab = ref<'html' | 'text' | 'raw' | 'attachments'>('html')
const textBody = ref('')
const rawBody = ref('')
const showRemote = ref(false)
const error = ref('')

const htmlSrc = computed(() => email.value ? api.htmlURL(email.value.sha256, showRemote.value) : '')

const tabs = computed(() => [
  { key: 'html' as const, label: t('viewer.tabs.html') },
  { key: 'text' as const, label: t('viewer.tabs.text') },
  { key: 'attachments' as const, label: t('viewer.tabs.attachments') + (parts.value ? ` (${parts.value.attachments.length})` : '') },
  { key: 'raw' as const, label: t('viewer.tabs.raw') },
])

async function load() {
  error.value = ''
  textBody.value = ''
  rawBody.value = ''
  try {
    const [e, p] = await Promise.all([api.getEmail(props.sha), api.getParts(props.sha)])
    email.value = e
    parts.value = p
    if (p.has_html) tab.value = 'html'
    else if (p.has_text) tab.value = 'text'
    else if (p.attachments.length > 0) tab.value = 'attachments'
    else tab.value = 'raw'
  } catch (err) {
    error.value = String(err)
  }
}

watch(() => props.sha, load, { immediate: true })

watch([tab, () => email.value?.sha256], async ([tb, sha]) => {
  if (!sha) return
  if (tb === 'text' && !textBody.value) textBody.value = await api.getText(sha)
  if (tb === 'raw' && !rawBody.value) rawBody.value = await api.getRaw(sha)
})

watch(() => email.value?.subject, (subject) => {
  if (props.standalone) document.title = subject ? `${subject} — Local Eml` : 'Local Eml'
})

function popOut() {
  window.open(`/email/${props.sha}`, '_blank', 'popup,width=820,height=900')
}

async function onTagAdd(tag: AcceptableInputValue) {
  if (!email.value) return
  const trimmed = String(tag).trim()
  if (!trimmed) return
  try {
    await api.addTag(email.value.sha256, trimmed)
    email.value.tags = [...email.value.tags, trimmed].sort()
  } catch (e) {
    toast.error(t('viewer.tag_invalid'), { description: String(e) })
  }
}

async function onTagRemove(tag: AcceptableInputValue) {
  if (!email.value) return
  const name = String(tag)
  try {
    await api.removeTag(email.value.sha256, name)
    email.value.tags = email.value.tags.filter((x) => x !== name)
  } catch (e) {
    toast.error(t('viewer.tag_remove_failed'), { description: String(e) })
  }
}
</script>

<template>
  <div v-if="error" class="text-destructive">{{ error }}</div>

  <div v-else-if="email" class="space-y-4">
    <Card class="p-5">
      <div class="flex items-start gap-3 mb-3">
        <h1 class="text-xl font-semibold flex-1 min-w-0">{{ email.subject || t('library.no_subject') }}</h1>
        <button
          v-if="!standalone"
          @click="popOut"
          class="shrink-0 text-xs text-muted-foreground hover:text-foreground rounded-sm px-2 py-1 hover:bg-accent"
        >
          {{ t('viewer.pop_out') }}
        </button>
      </div>
      <dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
        <dt class="text-muted-foreground">{{ t('viewer.from') }}</dt><dd>{{ email.from }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.to') }}</dt><dd>{{ email.to.join(', ') }}</dd>
        <dt v-if="email.cc.length" class="text-muted-foreground">{{ t('viewer.cc') }}</dt><dd v-if="email.cc.length">{{ email.cc.join(', ') }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.date') }}</dt><dd>{{ formatDate(email.sent_at) }}</dd>
        <dt class="text-muted-foreground">{{ t('viewer.size') }}</dt><dd>{{ formatBytes(email.size_bytes) }}</dd>
      </dl>

      <div class="mt-4">
        <div class="text-xs uppercase tracking-wide text-muted-foreground mb-1.5">{{ t('viewer.tags') }}</div>
        <TagsInput
          :model-value="email.tags"
          :add-on-paste="true"
          :delimiter="/[,]/"
          @add-tag="onTagAdd"
          @remove-tag="onTagRemove"
        >
          <TagsInputItem v-for="tg in email.tags" :key="tg" :value="tg">
            <TagsInputItemText>{{ tg }}</TagsInputItemText>
            <TagsInputItemDelete />
          </TagsInputItem>
          <TagsInputInput :placeholder="t('viewer.add_tag')" />
        </TagsInput>
      </div>
    </Card>

    <div class="flex items-center gap-1 border-b">
      <button v-for="tb in tabs" :key="tb.key" @click="tab = tb.key"
        :class="['px-3 py-2 text-sm border-b-2 -mb-px',
          tab === tb.key ? 'border-foreground' : 'border-transparent text-muted-foreground hover:text-foreground']">
        {{ tb.label }}
      </button>
      <div class="ml-auto flex items-center gap-2 pb-1" v-if="tab === 'html'">
        <label class="text-xs text-muted-foreground flex items-center gap-1 cursor-pointer">
          <input type="checkbox" v-model="showRemote" /> {{ t('viewer.load_remote') }}
        </label>
      </div>
    </div>

    <div v-if="tab === 'html'" class="border rounded-lg overflow-hidden bg-white">
      <iframe v-if="parts?.has_html" :src="htmlSrc" sandbox="allow-same-origin"
        class="w-full h-[70vh]" referrerpolicy="no-referrer"></iframe>
      <p v-else class="p-6 text-muted-foreground">{{ t('viewer.no_html') }}</p>
    </div>

    <Card v-else-if="tab === 'text'" class="p-4">
      <pre v-if="parts?.has_text" class="whitespace-pre-wrap font-mono text-sm">{{ textBody }}</pre>
      <p v-else class="text-muted-foreground">{{ t('viewer.no_text') }}</p>
    </Card>

    <Card v-else-if="tab === 'raw'" class="p-4">
      <pre class="whitespace-pre-wrap font-mono text-xs max-h-[70vh] overflow-auto">{{ rawBody }}</pre>
    </Card>

    <Card v-else-if="tab === 'attachments'" class="p-4">
      <ul v-if="parts && parts.attachments.length" class="divide-y">
        <li v-for="a in parts.attachments" :key="a.index" class="py-2 flex items-center gap-3">
          <span class="font-medium flex-1">{{ a.filename || `attachment-${a.index}` }}</span>
          <span class="text-xs text-muted-foreground">{{ a.content_type }}</span>
          <span class="text-xs text-muted-foreground">{{ formatBytes(a.size) }}</span>
          <a :href="api.attachmentURL(email.sha256, a.index)" target="_blank"
            class="text-sm underline">{{ t('viewer.download') }}</a>
        </li>
      </ul>
      <p v-else class="text-muted-foreground">{{ t('viewer.no_attachments') }}</p>
    </Card>
  </div>

  <p v-else class="text-muted-foreground">{{ t('viewer.loading') }}</p>
</template>
```

- [ ] **Step 2: Type-check**

Run: `npm run type-check`
Expected: PASS (no errors). `EmailDetail.vue` compiles even though nothing imports it yet.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/EmailDetail.vue
git commit -m "feat(web): extract reusable EmailDetail component"
```

---

## Task 3: Slim `ViewerPage.vue` to a wrapper

**Files:**
- Modify: `web/src/pages/ViewerPage.vue` (full replace)

- [ ] **Step 1: Replace the entire file contents**

```vue
<script setup lang="ts">
import EmailDetail from '@/components/EmailDetail.vue'

defineProps<{ sha: string }>()
</script>

<template>
  <EmailDetail :sha="sha" standalone />
</template>
```

- [ ] **Step 2: Type-check**

Run: `npm run type-check`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/ViewerPage.vue
git commit -m "refactor(web): ViewerPage delegates to EmailDetail (standalone)"
```

---

## Task 4: Chrome-less route + `App.vue` layout branch

**Files:**
- Modify: `web/src/router.ts:5`
- Modify: `web/src/App.vue`

- [ ] **Step 1: Add `chromeless` meta to the viewer route**

In `web/src/router.ts`, change the viewer route line:

```ts
  { path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true, meta: { titleKey: 'nav.viewer' } },
```

to:

```ts
  { path: '/email/:sha', name: 'viewer', component: () => import('@/pages/ViewerPage.vue'), props: true, meta: { titleKey: 'nav.viewer', chromeless: true } },
```

- [ ] **Step 2: Replace `web/src/App.vue` to branch on `chromeless`**

```vue
<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { RouterLink, RouterView, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Toaster } from 'vue-sonner'
import 'vue-sonner/style.css'

const APP_NAME = 'Local Eml'
const { t } = useI18n()
const route = useRoute()

const chromeless = computed(() => route.meta.chromeless === true)

const nav = computed(() => [
  { to: '/', label: t('nav.library') },
  { to: '/import', label: t('nav.import') },
  { to: '/settings', label: t('nav.settings') },
])

const pageTitle = computed(() => {
  const key = route.meta.titleKey as string | undefined
  return key ? t(key) : ''
})

watchEffect(() => {
  // Chrome-less detail windows set their own title (the email subject)
  // from inside EmailDetail; skip the app-level title there.
  if (chromeless.value) return
  document.title = pageTitle.value
    ? `${APP_NAME} | ${pageTitle.value}`
    : APP_NAME
})
</script>

<template>
  <div v-if="chromeless" class="min-h-screen px-6 py-6">
    <RouterView />
    <Toaster position="bottom-right" rich-colors close-button />
  </div>

  <div v-else class="min-h-screen flex flex-col">
    <header class="bg-black text-white">
      <div class="max-w-[1800px] mx-auto px-6 h-11 flex items-center gap-6 text-xs">
        <RouterLink to="/" class="flex items-center gap-2 font-semibold tracking-tight text-white/90 hover:text-white">
          <img src="/icon-64.png" srcset="/favicon.png 1x, /icon-64.png 2x" alt="" class="h-6 w-6 rounded-sm" />
          <span>{{ APP_NAME }}</span>
        </RouterLink>
        <nav class="flex items-center gap-1">
          <RouterLink
            v-for="r in nav"
            :key="r.to"
            :to="r.to"
            class="px-3 py-1 rounded-xs text-white/70 hover:text-white"
            active-class="text-white"
          >
            {{ r.label }}
          </RouterLink>
        </nav>
      </div>
    </header>
    <main class="flex-1 max-w-[1800px] mx-auto w-full px-6 py-10">
      <RouterView />
    </main>
    <Toaster position="bottom-right" rich-colors close-button />
  </div>
</template>
```

- [ ] **Step 3: Type-check**

Run: `npm run type-check`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add web/src/router.ts web/src/App.vue
git commit -m "feat(web): chrome-less layout for /email/:sha detail windows"
```

---

## Task 5: Two-pane Library with URL-query state

**Files:**
- Modify: `web/src/pages/LibraryPage.vue` (replace the `<script setup>` block and the `<template>`; keep the trailing `<script lang="ts">` `Th` helper block unchanged)

The `<script setup>` derives all list state from `route.query` (single source of truth). The search box keeps a local edit buffer that debounce-pushes to the URL. Filters/sort/page use `push`; selection uses `replace`. The template adds the detail pane.

- [ ] **Step 1: Replace the `<script setup lang="ts">` block (lines 1–79)**

Replace everything from `<script setup lang="ts">` through its closing `</script>` (the first script block) with:

```vue
<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ChevronsLeft, ChevronsRight } from 'lucide-vue-next'
import { api, type Email, type Tag } from '@/lib/api'
import { formatBytes, formatDate, shortSHA } from '@/lib/format'
import { useDebounceFn, useStorage } from '@vueuse/core'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import EmailDetail from '@/components/EmailDetail.vue'
import {
  Sidebar,
  SidebarHeader,
  SidebarTitle,
  SidebarContent,
  SidebarMenu,
  SidebarMenuItem,
  SidebarMenuButton,
} from '@/components/ui/sidebar'

type SortCol = 'sent_at' | 'from_addr' | 'subject' | 'size_bytes'
type Order = 'asc' | 'desc'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const items = ref<Email[]>([])
const total = ref(0)
const limit = ref(50)
const loading = ref(false)
const tags = ref<Tag[]>([])
const sidebarOpen = useStorage('library-sidebar-open', true)

function str(v: unknown, def = ''): string {
  return typeof v === 'string' ? v : def
}

// route.query is the single source of truth for list state.
const q = computed(() => str(route.query.q))
const tag = computed(() => str(route.query.tag))
const sort = computed<SortCol>(() => str(route.query.sort, 'sent_at') as SortCol)
const order = computed<Order>(() => str(route.query.order, 'desc') as Order)
const offset = computed(() => Number(str(route.query.offset, '0')) || 0)
const selected = computed(() => str(route.query.sel))

// Local edit buffer for the search box; debounce-pushed to the URL.
const searchInput = ref(q.value)
const debouncedPushSearch = useDebounceFn((val: string) => {
  pushQuery({ q: val || undefined, offset: undefined })
}, 250)
watch(searchInput, (val) => debouncedPushSearch(val))
// Keep the box in sync when q changes externally (back/forward, link open).
watch(q, (val) => { if (val !== searchInput.value) searchInput.value = val })

function mergeQuery(patch: Record<string, string | undefined>): LocationQueryRaw {
  const next: Record<string, string> = {}
  for (const [k, v] of Object.entries(route.query)) {
    if (typeof v === 'string') next[k] = v
  }
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined || v === '') delete next[k]
    else next[k] = v
  }
  return next
}

function pushQuery(patch: Record<string, string | undefined>) {
  router.push({ query: mergeQuery(patch) })
}

function replaceQuery(patch: Record<string, string | undefined>) {
  router.replace({ query: mergeQuery(patch) })
}

async function load() {
  loading.value = true
  try {
    const r = await api.listEmails({
      q: q.value || undefined,
      tag: tag.value || undefined,
      sort: sort.value,
      order: order.value,
      limit: limit.value,
      offset: offset.value,
    })
    items.value = r.items
    total.value = r.total
  } finally {
    loading.value = false
  }
}

// Reload whenever any list-affecting query param changes (not selection).
watch([q, tag, sort, order, offset], load)

onMounted(async () => {
  tags.value = await api.listTags()
  await load()
})

function setSort(col: SortCol) {
  if (sort.value === col) pushQuery({ sort: col, order: order.value === 'asc' ? 'desc' : 'asc' })
  else pushQuery({ sort: col, order: 'desc' })
}

function setTag(name: string) {
  pushQuery({ tag: name || undefined, offset: undefined })
}

function setOffset(value: number) {
  pushQuery({ offset: value > 0 ? String(value) : undefined })
}

function select(sha: string) {
  replaceQuery({ sel: sha })
}

const pageInfo = computed(() => {
  if (total.value === 0) return t('library.page_count_zero')
  const end = Math.min(offset.value + items.value.length, total.value)
  return t('library.page_count', { start: offset.value + 1, end, total: total.value })
})
</script>
```

- [ ] **Step 2: Replace the `<template>` block**

Replace the entire `<template>...</template>` with:

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

    <section class="flex-1 min-w-0">
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

    <aside class="w-[40rem] shrink-0 min-w-0">
      <EmailDetail v-if="selected" :sha="selected" />
      <Card v-else class="p-10 text-center text-muted-foreground">
        {{ t('library.select_prompt') }}
      </Card>
    </aside>
  </div>
</template>
```

- [ ] **Step 3: Type-check**

Run: `npm run type-check`
Expected: PASS. (The trailing `<script lang="ts">` `Th` helper block is unchanged and still compiles.)

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/LibraryPage.vue
git commit -m "feat(web): two-pane master-detail library with URL-query state"
```

---

## Manual Verification (maintainer)

Run `make build` then `./local-eml serve` (or `make web-dev` for the Vite dev server) and check:

1. Click a row → detail loads in the right pane; the list does not navigate away and keeps its scroll/filters.
2. Type a search, click a tag, sort a column, page next → the URL query updates (`?q=…&tag=…&sort=…&order=…&offset=…`). Reload → the same list and selection are restored.
3. Browser back/forward → steps through filter/sort/page changes (not through every row click).
4. Copy the URL into a new tab → same filtered list with the same email selected.
5. Click **⧉ Open in new window** → a chrome-less window opens showing only the detail, its OS title is the email subject; the library keeps its selection; a second click opens another window.
6. In both the in-pane detail and the popped-out window: HTML/Text/Raw/Attachments tabs, the remote-image toggle, tag add/remove, and attachment download all work.
7. With nothing selected (`/` with no `sel`), the right pane shows "Select an email to read."

Backend is unchanged, so `gofmt`, `go vet`, and `go test ./...` remain green.

---

## Self-Review

- **Spec coverage:** §Component decomposition → Tasks 2,3,5. §Routing & chrome-less window → Task 4 (+ pop-out button/title in Task 2). §State in the URL → Task 5 (`push` for filters/sort/page via `pushQuery`, `replace` for selection via `replaceQuery`, debounced search, offset reset on q/tag). §Removed/unchanged → `viewer.back` removed (Task 1), back link absent from `EmailDetail` (Task 2). §i18n keys → Task 1.
- **Type consistency:** `EmailDetail` props `{ sha: string; standalone?: boolean }` used identically in `ViewerPage` (Task 3, `standalone`) and `LibraryPage` (Task 5, no flag → in-pane with pop-out). `SortCol`/`Order` types match the `Th` helper's string props. `pushQuery`/`replaceQuery`/`mergeQuery`/`setSort`/`setTag`/`setOffset`/`select` all defined in Task 5 and referenced consistently in its template.
- **Placeholder scan:** none — every step has full file content or an exact before/after edit.
