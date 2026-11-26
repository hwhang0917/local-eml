<script setup lang="ts">
import { ref, watch, onBeforeUpdate, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { Check, ChevronDown, ChevronRight, Columns3, CornerDownRight, Mail, MailOpen, MessagesSquare, RotateCcw, Star, Trash2, TriangleAlert, X } from 'lucide-vue-next'
import {
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItemIndicator,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  TooltipContent,
  TooltipPortal,
  TooltipProvider,
  TooltipRoot,
  TooltipTrigger,
} from 'reka-ui'
import { api, type BulkAction, type Email, type EmailListParams, type PartInfo } from '@/lib/api'
import { attachmentName, dateFormat, formatBytes, formatDate, formatDateAbsolute, riskFor, senderName, shortSHA } from '@/lib/format'
import { useDebounceFn, useEventListener, useStorage } from '@vueuse/core'
import { PAGE_SIZES, useListQuery } from '@/composables/useListQuery'
import { useTour } from '@/composables/useTour'
import { ANY_CATEGORY, NO_CATEGORY, parseCategoryValue, useCategories } from '@/composables/useCategories'
import { useEmailActions } from '@/composables/useEmailActions'
import { useReadTracking } from '@/composables/useReadTracking'
import { useListContext } from '@/composables/useListContext'
import { onMailSignal } from '@/composables/useMailEvents'
import { useSession } from '@/composables/useSession'
import { hasModifier, isTypingTarget } from '@/lib/keys'
import { highlightTerms } from '@/lib/searchQuery'
import { cn } from '@/lib/utils'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import ConfirmDialog from '@/components/ui/ConfirmDialog.vue'
import CategoryDot from '@/components/ui/CategoryDot.vue'
import CategoryMenu, { type CategoryOption } from '@/components/ui/CategoryMenu.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import Highlight from '@/components/ui/Highlight.vue'
import SearchHelp from '@/components/SearchHelp.vue'
import PageNav from '@/components/ui/PageNav.vue'
import SortHeader from '@/components/ui/SortHeader.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const SEARCH_DEBOUNCE_MS = 250

type SortCol = 'sent_at' | 'from_addr' | 'subject' | 'size_bytes'
type Order = NonNullable<EmailListParams['order']>

const router = useRouter()
const {
  param, pushQuery, offset, limit, goPrevPage, goNextPage, setPageSize, setDateRange, pageLabel,
} = useListQuery('library-page-size')
const { t } = useI18n()

const items = ref<Email[]>([])
const total = ref(0)
const loading = ref(false)

// Subject is not hideable: it's the row's identity and its link to the viewer.
const HIDEABLE_COLS = ['starred', 'attachments', 'category', 'date', 'from', 'size'] as const
type HideableCol = (typeof HIDEABLE_COLS)[number]
const hiddenCols = useStorage<HideableCol[]>('library-hidden-columns', [])
function colShown(c: HideableCol) {
  return !hiddenCols.value.includes(c)
}
function toggleCol(c: HideableCol) {
  hiddenCols.value = colShown(c) ? [...hiddenCols.value, c] : hiddenCols.value.filter((x) => x !== c)
}
// Counted from the whitelist, not hiddenCols.length, so a stale storage entry
// can't skew the colspan.
// The read column is not in HIDEABLE_COLS: it exists only while read tracking is on.
const { enabled: readTracking } = useReadTracking()
// The leading 2 are the selection checkbox and the subject.
const colCount = computed(() => 2 + HIDEABLE_COLS.filter(colShown).length + (readTracking.value ? 1 : 0))

const q = computed(() => param('q'))
const starredOnly = computed(() => param('starred') === '1')
const unreadOnly = computed(() => param('unread') === '1')
const category = computed(() => param('category'))
const from = computed(() => param('from'))
const to = computed(() => param('to'))
const sort = computed<SortCol>(() => param('sort', 'sent_at') as SortCol)
const order = computed<Order>(() => param('order', 'desc') as Order)

const searchInput = ref(q.value)
let lastPushedQ: string | null = null
const debouncedPushSearch = useDebounceFn((val: string) => {
  lastPushedQ = val
  pushQuery({ q: val || undefined, offset: undefined })
}, SEARCH_DEBOUNCE_MS)
watch(searchInput, (val) => debouncedPushSearch(val))
watch(q, (val) => {
  if (val === lastPushedQ) return
  if (val !== searchInput.value) searchInput.value = val
})
// Operators aren't text to find, so each column highlights only the plain
// words and the operator aimed at it (subject:, from:).
const subjectHighlight = computed(() => highlightTerms(q.value, 'subject'))
const fromHighlight = computed(() => highlightTerms(q.value, 'from'))

// Filter and sort only; the page (limit/offset) is added where it matters, so
// the viewer's prev/next can re-run the same query around any index.
const listParams = computed<EmailListParams>(() => ({
  q: q.value || undefined,
  starred: starredOnly.value || undefined,
  unread: unreadOnly.value || undefined,
  category: category.value || undefined,
  from: from.value || undefined,
  to: to.value || undefined,
  sort: sort.value,
  order: order.value,
  group: groupThreads.value ? 'thread' : undefined,
}))
// The query for the page on screen.
const pageParams = computed<EmailListParams>(() => ({ ...listParams.value, limit: limit.value, offset: offset.value }))

async function load() {
  loading.value = true
  expandedThreads.value.clear()
  // A selection belongs to the page it was made on; a new query is a new page.
  selected.value.clear()
  try {
    const r = await api.listEmails(pageParams.value)
    items.value = r.items
    total.value = r.total
  } catch (e) {
    toast.error(t('library.load_error'), { description: String(e) })
  } finally {
    loading.value = false
  }
}

// Grouped mode collapses each conversation to its newest message; expanding a
// row fetches the rest through the same endpoint the viewer's card uses.
const groupThreads = useStorage('library-group-threads', true)
const expandedThreads = ref(new Map<string, Email[]>())

function toggleGroupThreads() {
  groupThreads.value = !groupThreads.value
  if (offset.value > 0) pushQuery({ offset: undefined })
  else load()
}

async function toggleThread(e: Email) {
  const m = expandedThreads.value
  if (m.has(e.sha256)) {
    m.delete(e.sha256)
    return
  }
  m.set(e.sha256, [])
  try {
    const r = await api.getThread(e.sha256)
    m.set(e.sha256, r.items.filter((x) => x.sha256 !== e.sha256))
  } catch {
    m.delete(e.sha256)
  }
}

watch([q, starredOnly, unreadOnly, category, from, to, sort, order, offset, limit], load)

// Live refresh. When the server says mail came in (a background sync, or an
// import in another tab), the page re-runs its query in place: no spinner,
// and the selection and open threads survive. Rows that weren't there before
// are tinted for a moment so the eye finds them.
const REFRESH_DEBOUNCE_MS = 400
const FRESH_ROW_MS = 4_000
const freshRows = ref(new Set<string>())
const liveAnnouncement = ref('')
let freshTimer: number | undefined

async function refreshInPlace() {
  if (loading.value || bulkBusy.value) {
    // Let the running load or bulk action land first, then look again.
    refreshSoon()
    return
  }
  const params = pageParams.value
  const asked = JSON.stringify(params)
  const before = new Set(items.value.map((e) => e.sha256))
  let r
  try {
    r = await api.listEmails(params)
  } catch {
    return // the next event or a manual reload will try again
  }
  // The person moved to another query meanwhile; its own load wins.
  if (loading.value || JSON.stringify(pageParams.value) !== asked) return

  const now = new Set(r.items.map((e) => e.sha256))
  items.value = r.items
  total.value = r.total
  for (const sha of selected.value) if (!now.has(sha)) selected.value.delete(sha)
  for (const sha of expandedThreads.value.keys()) if (!now.has(sha)) expandedThreads.value.delete(sha)

  const added = r.items.filter((e) => !before.has(e.sha256)).map((e) => e.sha256)
  if (added.length === 0) return
  freshRows.value = new Set(added)
  window.clearTimeout(freshTimer)
  freshTimer = window.setTimeout(() => freshRows.value.clear(), FRESH_ROW_MS)
  liveAnnouncement.value = t('library.new_mail_arrived', { count: added.length }, added.length)
}
const refreshSoon = useDebounceFn(refreshInPlace, REFRESH_DEBOUNCE_MS)

const stopMailSignal = onMailSignal(() => void refreshSoon())
onUnmounted(() => {
  stopMailSignal()
  window.clearTimeout(freshTimer)
})

const tour = useTour()
onMounted(() => {
  load()
  tour.startIfFirstVisit()
})
useEventListener(window, 'keydown', onKeydown)

// j/k moves real focus across the subject links: screen readers announce the
// row, Enter opens natively, and the focus-visible ring is the cursor.
const rowLinks = ref<HTMLElement[]>([])
onBeforeUpdate(() => {
  rowLinks.value = []
})
function setRowLink(el: unknown, i: number) {
  const node = (el as { $el?: HTMLElement } | null)?.$el ?? (el as HTMLElement | null)
  if (node) rowLinks.value[i] = node
}
const searchEl = ref<InstanceType<typeof Input> | null>(null)

function focusedRow(): number {
  return rowLinks.value.findIndex((el) => el === document.activeElement)
}

function focusRow(i: number) {
  const el = rowLinks.value[i]
  if (!el) return
  el.focus()
  el.scrollIntoView({ block: 'nearest' })
}

function onKeydown(e: KeyboardEvent) {
  if (hasModifier(e)) return
  if (e.key === 'Escape') {
    const target = e.target as HTMLElement | null
    if (target && isTypingTarget(e)) {
      if (target === searchInputEl()) searchInput.value = ''
      target.blur()
    }
    return
  }
  if (isTypingTarget(e)) return
  switch (e.key) {
    case 'j':
    case 'k': {
      if (items.value.length === 0) return
      e.preventDefault()
      const cur = focusedRow()
      const next = e.key === 'j'
        ? Math.min(cur + 1, items.value.length - 1)
        : Math.max(cur - 1, 0)
      focusRow(next)
      break
    }
    case 's': {
      const cur = focusedRow()
      if (cur >= 0) toggleStar(items.value[cur])
      break
    }
    case 'x': {
      const cur = focusedRow()
      if (cur >= 0) toggleSelected(items.value[cur].sha256)
      break
    }
    case '/':
      e.preventDefault()
      searchInputEl()?.focus()
      break
  }
}

// Adds an operator from the search help and leaves the caret after it, ready
// for the value ("from:" waits for a name; "has:attachment" is complete).
function insertOperator(text: string) {
  const cur = searchInput.value.trimEnd()
  const complete = !text.endsWith(':')
  searchInput.value = (cur ? cur + ' ' : '') + text + (complete ? ' ' : '')
  requestAnimationFrame(() => {
    const el = searchInputEl()
    if (!el) return
    el.focus()
    el.setSelectionRange(el.value.length, el.value.length)
  })
}

function searchInputEl(): HTMLInputElement | null {
  const el = (searchEl.value as { $el?: HTMLElement } | null)?.$el
  return el instanceof HTMLInputElement ? el : (el?.querySelector('input') ?? null)
}

const listCtx = useListContext()
function rememberPosition(rowIndex: number) {
  listCtx.set({
    params: listParams.value,
    index: offset.value + rowIndex,
    total: total.value,
  })
}

function setSort(col: SortCol) {
  if (sort.value === col) pushQuery({ sort: col, order: order.value === 'asc' ? 'desc' : 'asc', offset: undefined })
  else pushQuery({ sort: col, order: 'desc', offset: undefined })
}

const canPrevPage = computed(() => offset.value > 0)
const canNextPage = computed(() => offset.value + limit.value < total.value)

function toggleStarredFilter() {
  pushQuery({ starred: starredOnly.value ? undefined : '1', offset: undefined })
}

function toggleUnreadFilter() {
  pushQuery({ unread: unreadOnly.value ? undefined : '1', offset: undefined })
}

const hasActiveFilters = computed(
  () =>
    q.value !== '' ||
    starredOnly.value ||
    unreadOnly.value ||
    category.value !== '' ||
    from.value !== '' ||
    to.value !== '' ||
    sort.value !== 'sent_at' ||
    order.value !== 'desc' ||
    offset.value > 0,
)

function resetFilters() {
  searchInput.value = ''
  lastPushedQ = ''
  pushQuery({
    q: undefined,
    starred: undefined,
    unread: undefined,
    category: undefined,
    from: undefined,
    to: undefined,
    sort: undefined,
    order: undefined,
    offset: undefined,
  })
}

function setCategoryFilter(v: string) {
  pushQuery({ category: v === ANY_CATEGORY ? undefined : v, offset: undefined })
}

function emailHref(sha: string) {
  return router.resolve({ name: 'viewer', params: { sha } }).href
}

function openEmail(sha: string, ev: MouseEvent) {
  // Rows aren't anchors, so the modifier conventions browsers give links for
  // free have to be reimplemented here.
  if (ev.ctrlKey || ev.metaKey || ev.shiftKey) {
    window.open(emailHref(sha), '_blank', 'noopener')
    return
  }
  router.push({ name: 'viewer', params: { sha } })
}

function openEmailAux(sha: string, ev: MouseEvent) {
  if (ev.button !== 1) return
  ev.preventDefault()
  window.open(emailHref(sha), '_blank', 'noopener')
}

const actions = useEmailActions()

// A row that no longer matches the active filter has to leave, or the list
// shows a lie until the next fetch.
function dropRow(e: Email) {
  items.value = items.value.filter((x) => x.sha256 !== e.sha256)
  total.value = Math.max(0, total.value - 1)
}

async function toggleStar(e: Email) {
  const next = !e.starred
  if (await actions.toggleStar(e) && starredOnly.value && !next) dropRow(e)
}

async function toggleRead(e: Email) {
  const next = !e.read
  if (await actions.toggleRead(e) && unreadOnly.value && next) dropRow(e)
}

// Bulk selection. Keyed by sha, holding top-level rows only: in grouped mode a
// row stands for its conversation, and the server widens it to every member.
const selected = ref(new Set<string>())
const allSelected = computed(
  () => items.value.length > 0 && items.value.every((e) => selected.value.has(e.sha256)),
)
const someSelected = computed(() => selected.value.size > 0 && !allSelected.value)
const selectionHasThreads = computed(
  () => groupThreads.value && items.value.some((e) => selected.value.has(e.sha256) && (e.thread_count ?? 0) > 1),
)
const bulkBusy = ref(false)
const { canDeleteMail: canDelete } = useSession()
// Compact so the action row fits the header's height; Button's sizes don't go this small.
const bulkBtn = 'inline-flex h-7 items-center gap-1.5 rounded-sm border border-hairline bg-pearl px-2.5 text-xs '
  + 'text-foreground hover:bg-accent disabled:opacity-50 disabled:pointer-events-none '
  + 'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'
const deleteConfirmOpen = ref(false)

function toggleSelected(sha: string) {
  const s = selected.value
  if (s.has(sha)) s.delete(sha)
  else s.add(sha)
}

function toggleSelectPage() {
  if (allSelected.value) selected.value.clear()
  else for (const e of items.value) selected.value.add(e.sha256)
}

async function runBulk(action: BulkAction) {
  const shas = [...selected.value]
  if (shas.length === 0 || bulkBusy.value) return
  bulkBusy.value = true
  try {
    const r = await api.bulkEmails(action, shas, groupThreads.value)
    toast.success(t(`library.bulk_${action}_done`, { count: r.affected }, r.affected))
    // Refetch rather than patch rows: deletes shift the page and a read change
    // can move rows in or out of the unread filter.
    await load()
    if (items.value.length === 0 && offset.value > 0) goPrevPage()
  } catch (err) {
    toast.error(t('library.bulk_error'), { description: String(err) })
  } finally {
    bulkBusy.value = false
  }
}

const { byId, labelFor, options: assignOptions, load: loadCategories } = useCategories()
onMounted(loadCategories)

const filterOptions = computed<CategoryOption[]>(() => [
  { value: ANY_CATEGORY, label: t('library.category_any') },
  ...assignOptions.value,
])
const filterLabel = computed(
  () => filterOptions.value.find((o) => o.value === (category.value || ANY_CATEGORY))?.label ?? '',
)
const filterColor = computed(
  () => filterOptions.value.find((o) => o.value === category.value)?.color,
)

async function setCategory(e: Email, value: string) {
  if (!await actions.setCategory(e, value)) return
  const next = parseCategoryValue(value)
  const stillMatches =
    category.value === '' ||
    (category.value === NO_CATEGORY ? next === null : String(next) === category.value)
  if (!stillMatches) dropRow(e)
}

// Attachment names aren't in the list response (the DB only stores a boolean),
// so the 📎 tooltip fetches the parts manifest on first hover and caches it.
const attachParts = ref(new Map<string, PartInfo[]>())
async function loadAttachments(sha: string) {
  if (attachParts.value.has(sha)) return
  try {
    const p = await api.getParts(sha)
    attachParts.value.set(sha, p.attachments)
  } catch {
    // leave the cache empty; the tooltip keeps its loading text
  }
}

const pageInfo = computed(() => pageLabel(items.value.length, total.value))
</script>

<template>
  <section class="flex-1 min-w-0">
    <div class="flex items-center flex-wrap gap-3 mb-4">
      <div class="relative w-full max-w-md" data-tour="search">
        <Input
          ref="searchEl"
          v-model="searchInput"
          type="search"
          :placeholder="t('library.search_placeholder')"
          :aria-label="t('library.search_placeholder')"
          class="w-full pr-16 [&::-webkit-search-cancel-button]:appearance-none"
        />
        <div class="absolute right-2 top-1/2 flex -translate-y-1/2 items-center gap-0.5">
          <button
            v-if="searchInput"
            type="button"
            :aria-label="t('library.clear_search')"
            :title="t('library.clear_search')"
            class="inline-flex h-6 w-6 items-center justify-center rounded-sm
              text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            @click="searchInput = ''"
          >
            <X class="h-4 w-4" />
          </button>
          <SearchHelp @insert="insertOperator" />
        </div>
      </div>
      <Button
        variant="outline"
        size="sm"
        data-tour="starred"
        :aria-pressed="starredOnly"
        :title="starredOnly ? t('library.show_all') : t('library.show_starred')"
        :class="starredOnly ? 'text-amber-500 border-amber-500' : ''"
        @click="toggleStarredFilter"
      >
        <Star class="h-4 w-4" :fill="starredOnly ? 'currentColor' : 'none'" />
        <span class="ml-1.5">{{ t('library.starred') }}</span>
      </Button>
      <Button
        v-if="readTracking"
        variant="outline"
        size="sm"
        :aria-pressed="unreadOnly"
        :title="unreadOnly ? t('library.show_all') : t('library.show_unread')"
        :class="unreadOnly ? 'text-primary border-primary' : ''"
        @click="toggleUnreadFilter"
      >
        <Mail class="h-4 w-4" />
        <span class="ml-1.5">{{ t('library.unread') }}</span>
      </Button>
      <DateRangePicker :from="from" :to="to" @change="setDateRange" />
      <CategoryMenu
        :model-value="category || ANY_CATEGORY"
        :options="filterOptions"
        @select="setCategoryFilter"
      >
        <template #trigger>
          <button
            type="button"
            data-tour="categories"
            :class="['inline-flex h-8 items-center gap-2 rounded-sm border border-hairline bg-pearl px-3 text-sm',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
              category ? 'text-foreground' : 'text-muted-foreground']"
          >
            <CategoryDot v-if="category" :color="filterColor" />
            <span>{{ filterLabel }}</span>
          </button>
        </template>
      </CategoryMenu>
      <Button
        variant="outline"
        size="sm"
        :title="t('library.reset')"
        :disabled="!hasActiveFilters"
        @click="resetFilters"
      >
        <RotateCcw class="h-4 w-4" />
        <span class="ml-1.5">{{ t('library.reset') }}</span>
      </Button>
      <div class="ml-auto flex items-center gap-3">
        <button
          type="button"
          :aria-pressed="groupThreads"
          :title="t('library.group_threads')"
          :aria-label="t('library.group_threads')"
          :class="['inline-flex h-8 w-8 items-center justify-center rounded-sm border',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
            groupThreads
              ? 'border-primary bg-primary text-primary-foreground'
              : 'border-hairline bg-pearl text-muted-foreground hover:text-foreground']"
          @click="toggleGroupThreads"
        >
          <MessagesSquare class="h-4 w-4" />
        </button>
        <DropdownMenuRoot>
          <DropdownMenuTrigger
            :title="t('library.columns')"
            :aria-label="t('library.columns')"
            class="inline-flex h-8 w-8 items-center justify-center rounded-sm border border-hairline bg-pearl
              text-muted-foreground hover:text-foreground
              focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Columns3 class="h-4 w-4" />
          </DropdownMenuTrigger>
          <DropdownMenuPortal>
            <DropdownMenuContent
              :side-offset="6"
              align="end"
              class="z-50 min-w-44 rounded-lg border border-hairline bg-background p-1 shadow-lg"
            >
              <DropdownMenuCheckboxItem
                v-for="c in HIDEABLE_COLS"
                :key="c"
                :model-value="colShown(c)"
                class="flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none
                  data-[highlighted]:bg-accent"
                @update:model-value="toggleCol(c)"
                @select.prevent
              >
                <span class="inline-flex h-4 w-4 items-center justify-center">
                  <DropdownMenuItemIndicator>
                    <Check class="h-4 w-4" />
                  </DropdownMenuItemIndicator>
                </span>
                <span>{{ t(`library.col.${c}`) }}</span>
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenuPortal>
        </DropdownMenuRoot>
        <label class="flex items-center gap-2 text-sm text-muted-foreground">
          <span>{{ t('library.per_page') }}</span>
          <Select :model-value="String(limit)" @update:model-value="(v) => setPageSize(v)">
            <SelectTrigger class="w-24 h-8">
              <SelectValue>{{ limit }}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="size in PAGE_SIZES" :key="size" :value="String(size)">
                {{ size }}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>
        <PageNav
          :info="pageInfo"
          :can-prev="canPrevPage"
          :can-next="canNextPage"
          @prev="goPrevPage"
          @next="goNextPage"
        />
      </div>
    </div>

    <p class="sr-only" aria-live="polite">{{ liveAnnouncement }}</p>
    <Card class="overflow-hidden" :aria-busy="loading">
      <table class="w-full text-sm">
        <caption class="sr-only">{{ t('library.table_caption') }}</caption>
        <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
          <!-- With a selection the column headers are hidden, not removed, and the
               bulk actions sit over them: the row keeps its height and every
               column its width, so nothing below moves. -->
          <tr :class="['h-10 relative', selected.size ? '[&>th:not(:first-child)]:invisible' : '']">
            <th scope="col" class="text-left px-3 py-2 w-10">
              <input
                type="checkbox"
                class="h-4 w-4 align-middle accent-primary cursor-pointer disabled:cursor-default"
                :checked="allSelected"
                :indeterminate="someSelected"
                :disabled="items.length === 0"
                :aria-label="t('library.select_page')"
                :title="t('library.select_page')"
                @change="toggleSelectPage"
              />
              <div
                v-if="selected.size"
                role="toolbar"
                :aria-label="t('library.bulk_actions')"
                class="absolute inset-y-0 left-10 right-0 flex items-center gap-2 whitespace-nowrap px-3
                  normal-case font-normal text-sm"
              >
                <span class="font-medium tabular-nums text-foreground">{{ t('library.selected_count', { count: selected.size }) }}</span>
                <span v-if="selectionHasThreads" class="hidden lg:inline truncate text-xs text-muted-foreground">
                  {{ t('library.selected_threads_hint') }}
                </span>
                <div class="ml-auto flex items-center gap-2">
                  <button v-if="readTracking" type="button" :class="bulkBtn" :disabled="bulkBusy" @click="runBulk('read')">
                    <MailOpen class="h-3.5 w-3.5" />
                    {{ t('library.bulk_mark_read') }}
                  </button>
                  <button v-if="readTracking" type="button" :class="bulkBtn" :disabled="bulkBusy" @click="runBulk('unread')">
                    <Mail class="h-3.5 w-3.5" />
                    {{ t('library.bulk_mark_unread') }}
                  </button>
                  <button
                    v-if="canDelete"
                    type="button"
                    :class="cn(bulkBtn, 'border-transparent bg-destructive text-destructive-foreground hover:bg-destructive/95')"
                    :disabled="bulkBusy"
                    @click="deleteConfirmOpen = true"
                  >
                    <Trash2 class="h-3.5 w-3.5" />
                    {{ t('library.bulk_delete') }}
                  </button>
                  <button
                    type="button"
                    :class="cn(bulkBtn, 'border-transparent bg-transparent')"
                    :title="t('library.clear_selection')"
                    :aria-label="t('library.clear_selection')"
                    :disabled="bulkBusy"
                    @click="selected.clear()"
                  >
                    <X class="h-4 w-4" />
                  </button>
                </div>
              </div>
            </th>
            <th v-if="colShown('starred')" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.starred') }}</span>
            </th>
            <th v-if="readTracking" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.read') }}</span>
            </th>
            <th v-if="colShown('attachments')" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.attachments') }}</span>
            </th>
            <th v-if="colShown('category')" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.category') }}</span>
            </th>
            <SortHeader v-if="colShown('date')" :label="t('library.col.date')" col="sent_at" :sort="sort" :order="order" @sort="setSort" />
            <SortHeader v-if="colShown('from')" :label="t('library.col.from')" col="from_addr" :sort="sort" :order="order" @sort="setSort" />
            <SortHeader :label="t('library.col.subject')" col="subject" :sort="sort" :order="order" @sort="setSort" />
            <SortHeader v-if="colShown('size')" :label="t('library.col.size')" col="size_bytes" :sort="sort" :order="order" @sort="setSort" align="right" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && items.length === 0">
            <td :colspan="colCount" class="px-3 py-6 text-center text-muted-foreground">{{ t('library.loading') }}</td>
          </tr>
          <tr v-else-if="items.length === 0">
            <td :colspan="colCount" class="px-3 py-6 text-center text-muted-foreground">
              <template v-if="starredOnly">{{ t('library.no_starred') }}</template>
              <template v-else-if="unreadOnly">{{ t('library.no_unread') }}</template>
              <template v-else>
                {{ t('library.no_emails') }}
                <RouterLink :to="{ name: 'import' }" class="underline">{{ t('library.import_some') }}</RouterLink>
              </template>
            </td>
          </tr>
          <template v-for="(e, i) in items" :key="e.sha256">
          <tr
            :class="['border-t hover:bg-accent/50 cursor-pointer', selected.has(e.sha256) ? 'bg-accent/30' : '', freshRows.has(e.sha256) ? 'row-fresh' : '']"
            @click="rememberPosition(i); openEmail(e.sha256, $event)"
            @auxclick="rememberPosition(i); openEmailAux(e.sha256, $event)"
            @mousedown.middle.prevent
          >
            <!-- The whole cell toggles, so a near miss doesn't open the email. -->
            <td class="px-3 py-2" @click.stop="toggleSelected(e.sha256)" @auxclick.stop>
              <input
                type="checkbox"
                class="h-4 w-4 align-middle accent-primary cursor-pointer"
                :checked="selected.has(e.sha256)"
                :aria-label="t('library.select_row')"
                @click.stop
                @change="toggleSelected(e.sha256)"
              />
            </td>
            <td v-if="colShown('starred')" class="px-3 py-2">
              <button
                type="button"
                :title="e.starred ? t('library.unstar') : t('library.star')"
                :aria-label="e.starred ? t('library.unstar') : t('library.star')"
                :class="['inline-flex items-center justify-center h-6 w-6 rounded-sm hover:bg-accent',
                  e.starred ? 'text-amber-500' : 'text-muted-foreground hover:text-foreground']"
                @click.stop="toggleStar(e)"
                @auxclick.stop
              >
                <Star class="h-4 w-4" :fill="e.starred ? 'currentColor' : 'none'" />
              </button>
            </td>
            <td v-if="readTracking" class="px-3 py-2">
              <button
                type="button"
                :title="e.read ? t('library.mark_unread') : t('library.mark_read')"
                :aria-label="e.read ? t('library.mark_unread') : t('library.mark_read')"
                :class="['inline-flex items-center justify-center h-6 w-6 rounded-sm hover:bg-accent',
                  e.read ? 'text-muted-foreground hover:text-foreground' : 'text-primary']"
                @click.stop="toggleRead(e)"
                @auxclick.stop
              >
                <component :is="e.read ? MailOpen : Mail" class="h-4 w-4" />
              </button>
            </td>
            <td v-if="colShown('attachments')" class="px-3 py-2 text-muted-foreground">
              <TooltipProvider v-if="e.has_attachments" :delay-duration="200">
                <TooltipRoot @update:open="(o: boolean) => o && loadAttachments(e.sha256)">
                  <TooltipTrigger as-child>
                    <span role="img" :aria-label="t('library.has_attachments')" class="cursor-default">📎</span>
                  </TooltipTrigger>
                  <TooltipPortal>
                    <TooltipContent
                      side="bottom"
                      :side-offset="4"
                      class="z-50 max-w-72 rounded-lg border border-hairline bg-background p-2 text-xs shadow-lg"
                    >
                      <template v-if="attachParts.has(e.sha256)">
                        <ul v-if="attachParts.get(e.sha256)!.length" class="space-y-1">
                          <li
                            v-for="a in attachParts.get(e.sha256)"
                            :key="a.index"
                            class="flex items-center gap-2"
                          >
                            <span class="truncate">{{ attachmentName(a) }}</span>
                            <span class="shrink-0 text-muted-foreground">{{ formatBytes(a.size) }}</span>
                          </li>
                        </ul>
                        <span v-else class="text-muted-foreground">{{ t('viewer.no_attachments') }}</span>
                      </template>
                      <span v-else class="text-muted-foreground">{{ t('library.loading') }}</span>
                    </TooltipContent>
                  </TooltipPortal>
                </TooltipRoot>
              </TooltipProvider>
            </td>
            <td v-if="colShown('category')" class="px-3 py-2">
              <CategoryMenu
                :model-value="e.category_id ? String(e.category_id) : NO_CATEGORY"
                :options="assignOptions"
                @select="(v) => setCategory(e, v)"
              >
                <template #trigger>
                  <button
                    type="button"
                    :aria-label="t('library.set_category')"
                    class="inline-flex h-6 w-6 items-center justify-center rounded-sm hover:bg-accent"
                    @click.stop
                    @auxclick.stop
                  >
                    <CategoryDot
                      size="md"
                      :color="e.category_id ? byId.get(e.category_id)?.color : undefined"
                      :name="e.category_id ? labelFor(byId.get(e.category_id)) : undefined"
                    />
                  </button>
                </template>
              </CategoryMenu>
            </td>
            <td v-if="colShown('date')" class="px-3 py-2 whitespace-nowrap text-muted-foreground">
              <time
                :datetime="e.sent_at || undefined"
                :title="dateFormat === 'relative' ? formatDateAbsolute(e.sent_at) : undefined"
              >{{ formatDate(e.sent_at) }}</time>
            </td>
            <td v-if="colShown('from')" class="px-3 py-2 whitespace-nowrap" :title="e.from">
              <Highlight :text="senderName(e.from)" :query="fromHighlight" />
            </td>
            <td class="px-3 py-2 truncate">
              <button
                v-if="(e.thread_count ?? 0) > 1"
                type="button"
                :aria-expanded="expandedThreads.has(e.sha256)"
                :aria-label="t('library.expand_thread', { count: e.thread_count })"
                :title="t('library.expand_thread', { count: e.thread_count })"
                class="mr-1.5 inline-flex items-center gap-0.5 rounded-sm border border-hairline bg-pearl px-1 py-0.5
                  align-middle text-xs tabular-nums text-muted-foreground hover:text-foreground
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                @click.stop="toggleThread(e)"
                @auxclick.stop
              >
                <component :is="expandedThreads.has(e.sha256) ? ChevronDown : ChevronRight" class="h-3 w-3" />
                {{ e.thread_count }}
              </button>
              <span
                v-if="riskFor(e).length"
                role="img"
                :aria-label="t('risk.title')"
                :title="t('risk.title') + '\n' + riskFor(e).map((r) => '• ' + t(`risk.${r.code}`)).join('\n')"
                class="mr-1.5 inline-flex align-middle text-amber-600 dark:text-amber-400"
              >
                <TriangleAlert class="h-3.5 w-3.5" />
              </span>
              <RouterLink
                :ref="(el) => setRowLink(el, i)"
                :to="{ name: 'viewer', params: { sha: e.sha256 } }"
                :class="['rounded-xs hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                  readTracking && !e.read ? 'font-semibold' : '']"
                @click.stop="rememberPosition(i)"
              >
                <Highlight v-if="e.subject" :text="e.subject" :query="subjectHighlight" />
                <template v-else>{{ t('library.no_subject') }}</template>
              </RouterLink>
              <!-- Full-strength muted token: the /60 variant computed to 2.4:1
                   contrast and failed WCAG AA in Lighthouse. -->
              <span class="ml-2 text-xs text-muted-foreground" :title="e.sha256">({{ shortSHA(e.sha256) }})</span>
            </td>
            <td v-if="colShown('size')" class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(e.size_bytes) }}</td>
          </tr>
          <tr
            v-for="m in expandedThreads.get(e.sha256) ?? []"
            :key="m.sha256"
            class="border-t bg-muted/20 hover:bg-accent/50 cursor-pointer"
            @click="openEmail(m.sha256, $event)"
            @auxclick="openEmailAux(m.sha256, $event)"
            @mousedown.middle.prevent
          >
            <td class="px-3 py-2"></td>
            <td v-if="colShown('starred')" class="px-3 py-2"></td>
            <td v-if="readTracking" class="px-3 py-2">
              <button
                type="button"
                :title="m.read ? t('library.mark_unread') : t('library.mark_read')"
                :aria-label="m.read ? t('library.mark_unread') : t('library.mark_read')"
                :class="['inline-flex items-center justify-center h-6 w-6 rounded-sm hover:bg-accent',
                  m.read ? 'text-muted-foreground hover:text-foreground' : 'text-primary']"
                @click.stop="actions.toggleRead(m)"
                @auxclick.stop
              >
                <component :is="m.read ? MailOpen : Mail" class="h-4 w-4" />
              </button>
            </td>
            <td v-if="colShown('attachments')" class="px-3 py-2 text-muted-foreground">
              <span v-if="m.has_attachments" role="img" :aria-label="t('library.has_attachments')">📎</span>
            </td>
            <td v-if="colShown('category')" class="px-3 py-2"></td>
            <td v-if="colShown('date')" class="px-3 py-2 whitespace-nowrap text-muted-foreground">
              <time :datetime="m.sent_at || undefined">{{ formatDate(m.sent_at) }}</time>
            </td>
            <td v-if="colShown('from')" class="px-3 py-2 whitespace-nowrap text-muted-foreground" :title="m.from">
              {{ senderName(m.from) }}
            </td>
            <td class="px-3 py-2 truncate">
              <span class="inline-flex max-w-full items-center gap-1.5 pl-5">
                <CornerDownRight class="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
                <RouterLink
                  :to="{ name: 'viewer', params: { sha: m.sha256 } }"
                  :class="['truncate rounded-xs hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                    readTracking && !m.read ? 'font-semibold' : '']"
                  @click.stop
                >
                  {{ m.subject || t('library.no_subject') }}
                </RouterLink>
              </span>
            </td>
            <td v-if="colShown('size')" class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(m.size_bytes) }}</td>
          </tr>
          </template>
        </tbody>
      </table>
    </Card>

    <div v-if="items.length" class="mt-4 flex justify-end">
      <PageNav
        :info="pageInfo"
        :can-prev="canPrevPage"
        :can-next="canNextPage"
        @prev="goPrevPage"
        @next="goNextPage"
      />
    </div>

    <ConfirmDialog
      v-model:open="deleteConfirmOpen"
      :title="selectionHasThreads
        ? t('library.bulk_delete_title_threads', { count: selected.size }, selected.size)
        : t('library.bulk_delete_title', { count: selected.size }, selected.size)"
      :description="t('library.bulk_delete_body')"
      :confirm-label="t('library.bulk_delete_confirm')"
      :cancel-label="t('library.bulk_delete_cancel')"
      destructive
      @confirm="runBulk('delete')"
    />
  </section>
</template>

