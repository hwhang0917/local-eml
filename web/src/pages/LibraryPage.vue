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
let lastPushedQ: string | null = null
const debouncedPushSearch = useDebounceFn((val: string) => {
  lastPushedQ = val
  pushQuery({ q: val || undefined, offset: undefined })
}, 250)
watch(searchInput, (val) => debouncedPushSearch(val))
// Sync the box only on external q changes (back/forward, link open),
// not on the echo of our own push (which could clobber newer keystrokes).
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
  if (sort.value === col) pushQuery({ sort: col, order: order.value === 'asc' ? 'desc' : 'asc', offset: undefined })
  else pushQuery({ sort: col, order: 'desc', offset: undefined })
}

function setTag(name: string) {
  pushQuery({ tag: name || undefined, offset: undefined })
}

function setOffset(value: number) {
  pushQuery({ offset: value > 0 ? String(value) : undefined })
}

function select(sha: string) {
  if (sha === selected.value) return
  replaceQuery({ sel: sha })
}

const pageInfo = computed(() => {
  if (total.value === 0) return t('library.page_count_zero')
  const end = Math.min(offset.value + items.value.length, total.value)
  return t('library.page_count', { start: offset.value + 1, end, total: total.value })
})
</script>

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
      return h(
        'th',
        {
          class: ['px-3 py-2 cursor-pointer select-none hover:text-foreground',
            props.align === 'right' ? 'text-right' : 'text-left'],
          onClick: () => emit('sort', props.col),
        },
        props.label + arrow,
      )
    }
  },
})
</script>
