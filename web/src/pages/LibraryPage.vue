<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Check, Columns3, RotateCcw, Star, X } from 'lucide-vue-next'
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
import { api, type Email, type PartInfo } from '@/lib/api'
import { dateFormat, formatBytes, formatDate, formatDateAbsolute, senderName, shortSHA } from '@/lib/format'
import { useDebounceFn, useStorage } from '@vueuse/core'
import { useTour } from '@/composables/useTour'
import { useCategories } from '@/composables/useCategories'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import CategoryDot from '@/components/ui/CategoryDot.vue'
import CategoryMenu, { type CategoryOption } from '@/components/ui/CategoryMenu.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import PageNav from '@/components/ui/PageNav.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

// 500 is the server's ceiling; anything larger is silently clamped back to 50.
const PAGE_SIZES = [10, 25, 50, 100, 200, 500] as const
type PageSize = (typeof PAGE_SIZES)[number]
const DEFAULT_PAGE_SIZE: PageSize = 50

type SortCol = 'sent_at' | 'from_addr' | 'subject' | 'size_bytes'
type Order = 'asc' | 'desc'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const items = ref<Email[]>([])
const total = ref(0)
const loading = ref(false)

const storedPageSize = useStorage<PageSize>('library-page-size', DEFAULT_PAGE_SIZE)

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
const colCount = computed(() => 1 + HIDEABLE_COLS.filter(colShown).length)

function str(v: unknown, def = ''): string {
  return typeof v === 'string' ? v : def
}

function normalizePageSize(n: number): PageSize {
  return (PAGE_SIZES as readonly number[]).includes(n) ? (n as PageSize) : DEFAULT_PAGE_SIZE
}

const q = computed(() => str(route.query.q))
const starredOnly = computed(() => str(route.query.starred) === '1')
const category = computed(() => str(route.query.category))
const from = computed(() => str(route.query.from))
const to = computed(() => str(route.query.to))
const sort = computed<SortCol>(() => str(route.query.sort, 'sent_at') as SortCol)
const order = computed<Order>(() => str(route.query.order, 'desc') as Order)
const offset = computed(() => Number(str(route.query.offset, '0')) || 0)
const limit = computed<PageSize>(() => {
  const fromQuery = Number(str(route.query.limit))
  if (fromQuery > 0) return normalizePageSize(fromQuery)
  return normalizePageSize(storedPageSize.value)
})

const searchInput = ref(q.value)
let lastPushedQ: string | null = null
const debouncedPushSearch = useDebounceFn((val: string) => {
  lastPushedQ = val
  pushQuery({ q: val || undefined, offset: undefined })
}, 250)
watch(searchInput, (val) => debouncedPushSearch(val))
watch(q, (val) => {
  if (val === lastPushedQ) return
  if (val !== searchInput.value) searchInput.value = val
})

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

async function load() {
  loading.value = true
  try {
    const r = await api.listEmails({
      q: q.value || undefined,
      starred: starredOnly.value || undefined,
      category: category.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
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

watch([q, starredOnly, category, from, to, sort, order, offset, limit], load)

const tour = useTour()
onMounted(() => {
  load()
  tour.startIfFirstVisit()
})

function setSort(col: SortCol) {
  if (sort.value === col) pushQuery({ sort: col, order: order.value === 'asc' ? 'desc' : 'asc', offset: undefined })
  else pushQuery({ sort: col, order: 'desc', offset: undefined })
}

function setOffset(value: number) {
  pushQuery({ offset: value > 0 ? String(value) : undefined })
}

const canPrevPage = computed(() => offset.value > 0)
const canNextPage = computed(() => offset.value + limit.value < total.value)

function goPrevPage() {
  setOffset(Math.max(0, offset.value - limit.value))
}

function goNextPage() {
  setOffset(offset.value + limit.value)
}

function toggleStarredFilter() {
  pushQuery({ starred: starredOnly.value ? undefined : '1', offset: undefined })
}

const hasActiveFilters = computed(
  () =>
    q.value !== '' ||
    starredOnly.value ||
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
    category: undefined,
    from: undefined,
    to: undefined,
    sort: undefined,
    order: undefined,
    offset: undefined,
  })
}

function setCategoryFilter(v: string) {
  pushQuery({ category: v === 'any' ? undefined : v, offset: undefined })
}

function setDateRange(range: { from: string; to: string }) {
  pushQuery({ from: range.from || undefined, to: range.to || undefined, offset: undefined })
}

function setPageSize(v: unknown) {
  if (v === null || v === undefined || v === '') return
  const size = normalizePageSize(Number(v))
  storedPageSize.value = size
  pushQuery({
    limit: size === DEFAULT_PAGE_SIZE ? undefined : String(size),
    offset: undefined,
  })
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

async function toggleStar(e: Email) {
  const next = !e.starred
  e.starred = next
  try {
    await api.setStarred(e.sha256, next)
    if (starredOnly.value && !next) {
      items.value = items.value.filter((x) => x.sha256 !== e.sha256)
      total.value = Math.max(0, total.value - 1)
    }
  } catch {
    e.starred = !next
  }
}

const { categories, byId, labelFor, load: loadCategories } = useCategories()
onMounted(loadCategories)

// "any" and "none" are sentinels rather than ids, so they survive the round trip
// through the URL alongside a numeric category id.
const filterOptions = computed<CategoryOption[]>(() => [
  { value: 'any', label: t('library.category_any') },
  { value: 'none', label: t('library.category_none') },
  ...categories.value.map((c) => ({ value: String(c.id), label: labelFor(c), color: c.color })),
])
const assignOptions = computed<CategoryOption[]>(() => [
  { value: 'none', label: t('library.category_none') },
  ...categories.value.map((c) => ({ value: String(c.id), label: labelFor(c), color: c.color })),
])
const filterLabel = computed(
  () => filterOptions.value.find((o) => o.value === (category.value || 'any'))?.label ?? '',
)
const filterColor = computed(
  () => filterOptions.value.find((o) => o.value === category.value)?.color,
)

async function setCategory(e: Email, value: string) {
  const next = value === 'none' ? null : Number(value)
  const prev = e.category_id
  e.category_id = next ?? undefined
  try {
    await api.setCategory(e.sha256, next)
    // Same as toggleStar: a row that no longer matches the active filter has to
    // leave, or the list shows a lie until the next fetch.
    const stillMatches =
      category.value === '' ||
      (category.value === 'none' ? next === null : String(next) === category.value)
    if (!stillMatches) {
      items.value = items.value.filter((x) => x.sha256 !== e.sha256)
      total.value = Math.max(0, total.value - 1)
    }
  } catch {
    e.category_id = prev
  }
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

const pageInfo = computed(() => {
  if (total.value === 0) return t('library.page_count_zero')
  const end = Math.min(offset.value + items.value.length, total.value)
  return t('library.page_count', { start: offset.value + 1, end, total: total.value })
})
</script>

<template>
  <section class="flex-1 min-w-0">
    <div class="flex items-center flex-wrap gap-3 mb-4">
      <div class="relative w-full max-w-md" data-tour="search">
        <Input v-model="searchInput" :placeholder="t('library.search_placeholder')" class="w-full pr-8" />
        <button
          v-if="searchInput"
          type="button"
          :aria-label="t('library.clear_search')"
          :title="t('library.clear_search')"
          class="absolute right-2 top-1/2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-sm
            text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @click="searchInput = ''"
        >
          <X class="h-4 w-4" />
        </button>
      </div>
      <Button
        variant="outline"
        size="sm"
        data-tour="starred"
        :title="starredOnly ? t('library.show_all') : t('library.show_starred')"
        :class="starredOnly ? 'text-amber-500 border-amber-500' : ''"
        @click="toggleStarredFilter"
      >
        <Star class="h-4 w-4" :fill="starredOnly ? 'currentColor' : 'none'" />
        <span class="ml-1.5">{{ t('library.starred') }}</span>
      </Button>
      <DateRangePicker :from="from" :to="to" @change="setDateRange" />
      <CategoryMenu
        :model-value="category || 'any'"
        :options="filterOptions"
        :label="t('library.category')"
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

    <Card class="overflow-hidden">
      <table class="w-full text-sm">
        <caption class="sr-only">{{ t('library.table_caption') }}</caption>
        <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
          <tr>
            <th v-if="colShown('starred')" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.starred') }}</span>
            </th>
            <th v-if="colShown('attachments')" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.attachments') }}</span>
            </th>
            <th v-if="colShown('category')" scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.category') }}</span>
            </th>
            <Th v-if="colShown('date')" :label="t('library.col.date')" col="sent_at" :sort="sort" :order="order" @sort="setSort" />
            <Th v-if="colShown('from')" :label="t('library.col.from')" col="from_addr" :sort="sort" :order="order" @sort="setSort" />
            <Th :label="t('library.col.subject')" col="subject" :sort="sort" :order="order" @sort="setSort" />
            <Th v-if="colShown('size')" :label="t('library.col.size')" col="size_bytes" :sort="sort" :order="order" @sort="setSort" align="right" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && items.length === 0">
            <td :colspan="colCount" class="px-3 py-6 text-center text-muted-foreground">{{ t('library.loading') }}</td>
          </tr>
          <tr v-else-if="items.length === 0">
            <td :colspan="colCount" class="px-3 py-6 text-center text-muted-foreground">
              <template v-if="starredOnly">{{ t('library.no_starred') }}</template>
              <template v-else>
                {{ t('library.no_emails') }}
                <RouterLink to="/import" class="underline">{{ t('library.import_some') }}</RouterLink>
              </template>
            </td>
          </tr>
          <tr
            v-for="e in items"
            :key="e.sha256"
            class="border-t hover:bg-accent/50 cursor-pointer"
            @click="openEmail(e.sha256, $event)"
            @auxclick="openEmailAux(e.sha256, $event)"
            @mousedown.middle.prevent
          >
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
                            <span class="truncate">{{ a.filename || `attachment-${a.index}` }}</span>
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
                :model-value="e.category_id ? String(e.category_id) : 'none'"
                :options="assignOptions"
                :label="t('library.set_category')"
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
              <Highlight :text="senderName(e.from)" :query="q" />
            </td>
            <td class="px-3 py-2 truncate">
              <RouterLink
                :to="{ name: 'viewer', params: { sha: e.sha256 } }"
                class="rounded-xs hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                @click.stop
              >
                <Highlight v-if="e.subject" :text="e.subject" :query="q" />
                <template v-else>{{ t('library.no_subject') }}</template>
              </RouterLink>
              <span class="ml-2 text-xs text-muted-foreground/60" :title="e.sha256">({{ shortSHA(e.sha256) }})</span>
            </td>
            <td v-if="colShown('size')" class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(e.size_bytes) }}</td>
          </tr>
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
  </section>
</template>

<script lang="ts">
import { defineComponent, h } from 'vue'
import { highlightSegments } from '@/lib/highlight'

// <mark> is the element the platform already has for "matched here", so
// find-in-page styling and assistive tech get it for free.
export const Highlight = defineComponent({
  props: {
    text: { type: String, required: true },
    query: { type: String, default: '' },
  },
  setup(props) {
    return () =>
      highlightSegments(props.text, props.query).map((seg) =>
        seg.match
          ? h('mark', { class: 'rounded-xs bg-amber-200 text-inherit dark:bg-amber-400/30' }, seg.text)
          : seg.text,
      )
  },
})

export const Th = defineComponent({
  props: {
    label: { type: String, required: true },
    col: { type: String, required: true },
    sort: { type: String, required: true },
    order: { type: String, required: true },
    align: { type: String, default: 'left' },
  },
  emits: ['sort'],
  setup(props, { emit }) {
    return () => {
      const active = props.sort === props.col
      const arrow = active ? (props.order === 'asc' ? ' ↑' : ' ↓') : ''
      // A bare <th onClick> is invisible to keyboards and screen readers. The
      // standard sortable-header pattern is a real button inside the th, with
      // aria-sort carrying the state the arrow shows visually.
      return h(
        'th',
        {
          scope: 'col',
          'aria-sort': active ? (props.order === 'asc' ? 'ascending' : 'descending') : 'none',
          class: ['px-3 py-2 select-none',
            props.align === 'right' ? 'text-right' : 'text-left'],
        },
        h(
          'button',
          {
            type: 'button',
            class: ['uppercase cursor-pointer hover:text-foreground rounded-xs',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring'],
            onClick: () => emit('sort', props.col),
          },
          [props.label, h('span', { 'aria-hidden': 'true' }, arrow)],
        ),
      )
    }
  },
})
</script>
