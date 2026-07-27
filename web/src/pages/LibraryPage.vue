<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { RotateCcw, Star } from 'lucide-vue-next'
import { api, type Email } from '@/lib/api'
import { dateFormat, formatBytes, formatDate, formatDateAbsolute, senderName, shortSHA } from '@/lib/format'
import { useDebounceFn, useStorage } from '@vueuse/core'
import { useTour } from '@/composables/useTour'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const PAGE_SIZES = [10, 25, 50, 100, 200] as const
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

function str(v: unknown, def = ''): string {
  return typeof v === 'string' ? v : def
}

function normalizePageSize(n: number): PageSize {
  return (PAGE_SIZES as readonly number[]).includes(n) ? (n as PageSize) : DEFAULT_PAGE_SIZE
}

const q = computed(() => str(route.query.q))
const starredOnly = computed(() => str(route.query.starred) === '1')
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

watch([q, starredOnly, sort, order, offset, limit], load)

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

function toggleStarredFilter() {
  pushQuery({ starred: starredOnly.value ? undefined : '1', offset: undefined })
}

const hasActiveFilters = computed(
  () =>
    q.value !== '' ||
    starredOnly.value ||
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
    sort: undefined,
    order: undefined,
    offset: undefined,
  })
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

const pageInfo = computed(() => {
  if (total.value === 0) return t('library.page_count_zero')
  const end = Math.min(offset.value + items.value.length, total.value)
  return t('library.page_count', { start: offset.value + 1, end, total: total.value })
})
</script>

<template>
  <section class="flex-1 min-w-0">
    <div class="flex items-center flex-wrap gap-3 mb-4">
      <Input v-model="searchInput" :placeholder="t('library.search_placeholder')" class="max-w-md" data-tour="search" />
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
        <label class="flex items-center gap-2 text-sm text-muted-foreground">
          <span>{{ t('library.per_page') }}</span>
          <Select :model-value="String(limit)" @update:model-value="(v) => setPageSize(v)">
            <SelectTrigger class="w-20 h-8">
              <SelectValue>{{ limit }}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="size in PAGE_SIZES" :key="size" :value="String(size)">
                {{ size }}
              </SelectItem>
            </SelectContent>
          </Select>
        </label>
        <span class="text-sm text-muted-foreground">{{ pageInfo }}</span>
        <div class="flex items-center gap-1">
          <Button variant="outline" size="sm" :disabled="offset === 0" @click="setOffset(Math.max(0, offset - limit))">{{ t('library.prev') }}</Button>
          <Button variant="outline" size="sm" :disabled="offset + limit >= total" @click="setOffset(offset + limit)">{{ t('library.next') }}</Button>
        </div>
      </div>
    </div>

    <Card class="overflow-hidden">
      <table class="w-full text-sm">
        <caption class="sr-only">{{ t('library.table_caption') }}</caption>
        <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
          <tr>
            <th scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.starred') }}</span>
            </th>
            <th scope="col" class="text-left px-3 py-2 w-10">
              <span class="sr-only">{{ t('library.col.attachments') }}</span>
            </th>
            <Th :label="t('library.col.date')" col="sent_at" :sort="sort" :order="order" @sort="setSort" />
            <Th :label="t('library.col.from')" col="from_addr" :sort="sort" :order="order" @sort="setSort" />
            <Th :label="t('library.col.subject')" col="subject" :sort="sort" :order="order" @sort="setSort" />
            <Th :label="t('library.col.size')" col="size_bytes" :sort="sort" :order="order" @sort="setSort" align="right" />
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && items.length === 0">
            <td colspan="6" class="px-3 py-6 text-center text-muted-foreground">{{ t('library.loading') }}</td>
          </tr>
          <tr v-else-if="items.length === 0">
            <td colspan="6" class="px-3 py-6 text-center text-muted-foreground">
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
            <td class="px-3 py-2">
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
            <td class="px-3 py-2 text-muted-foreground">
              <span v-if="e.has_attachments" role="img" :aria-label="t('library.has_attachments')">📎</span>
            </td>
            <td class="px-3 py-2 whitespace-nowrap text-muted-foreground">
              <time
                :datetime="e.sent_at || undefined"
                :title="dateFormat === 'relative' ? formatDateAbsolute(e.sent_at) : undefined"
              >{{ formatDate(e.sent_at) }}</time>
            </td>
            <td class="px-3 py-2 whitespace-nowrap" :title="e.from">{{ senderName(e.from) }}</td>
            <td class="px-3 py-2 truncate">
              <RouterLink
                :to="{ name: 'viewer', params: { sha: e.sha256 } }"
                class="rounded-xs hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                @click.stop
              >
                {{ e.subject || t('library.no_subject') }}
              </RouterLink>
              <span class="ml-2 text-xs text-muted-foreground">{{ shortSHA(e.sha256) }}</span>
            </td>
            <td class="px-3 py-2 text-right whitespace-nowrap text-muted-foreground">{{ formatBytes(e.size_bytes) }}</td>
          </tr>
        </tbody>
      </table>
    </Card>

  </section>
</template>

<script lang="ts">
import { defineComponent, h } from 'vue'

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
