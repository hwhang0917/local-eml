<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { useDebounceFn } from '@vueuse/core'
import {
  Download,
  ExternalLink,
  File,
  FileArchive,
  FileImage,
  FilePlay,
  FileSpreadsheet,
  FileText,
  FileType,
  Mail,
  RotateCcw,
  X,
} from 'lucide-vue-next'
import {
  api,
  ATTACHMENT_KINDS,
  type AttachmentItem,
  type AttachmentKind,
  type AttachmentListParams,
} from '@/lib/api'
import { docPreviewKind } from '@/lib/docPreview'
import { dateFormat, formatBytes, formatDate, formatDateAbsolute, senderName } from '@/lib/format'
import { onMailSignal } from '@/composables/useMailEvents'
import { atLeastLg, atLeastMd, atLeastXl } from '@/composables/useWidth'
import { PAGE_SIZES, useListQuery } from '@/composables/useListQuery'
import Input from '@/components/ui/Input.vue'
import Button from '@/components/ui/Button.vue'
import Card from '@/components/ui/Card.vue'
import DateRangePicker from '@/components/ui/DateRangePicker.vue'
import PageNav from '@/components/ui/PageNav.vue'
import SortHeader from '@/components/ui/SortHeader.vue'
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/ui/select'

const TYPING_DEBOUNCE_MS = 250
const ALL_KINDS = 'all'

type SortCol = NonNullable<AttachmentListParams['sort']>
type Order = NonNullable<AttachmentListParams['order']>

const { t, locale } = useI18n()
const n = (v: number) => v.toLocaleString(locale.value)

const items = ref<AttachmentItem[]>([])
const total = ref(0)
const totalBytes = ref(0)
const kindCounts = ref<Partial<Record<AttachmentKind, number>>>({})
const loading = ref(false)
const {
  param, pushQuery, offset, limit, goPrevPage, goNextPage, setPageSize, setDateRange, pageLabel,
} = useListQuery('attachments-page-size')

const q = computed(() => param('q'))
const sender = computed(() => param('sender'))
const kind = computed(() => {
  const k = param('kind')
  return (ATTACHMENT_KINDS as readonly string[]).includes(k) ? (k as AttachmentKind) : undefined
})
const from = computed(() => param('from'))
const to = computed(() => param('to'))
const sort = computed<SortCol>(() => param('sort', 'date') as SortCol)
const order = computed<Order>(() => param('order', 'desc') as Order)

// Typed filters update the URL after a pause; an outside change to the URL
// (back button, reset) flows back into the box.
function typedFilter(key: 'q' | 'sender', current: () => string) {
  const input = ref(current())
  const push = useDebounceFn((v: string) => pushQuery({ [key]: v || undefined, offset: undefined }), TYPING_DEBOUNCE_MS)
  watch(input, (v) => {
    if (v !== current()) push(v)
  })
  watch(current, (v) => {
    if (v !== input.value) input.value = v
  })
  return input
}
const nameInput = typedFilter('q', () => q.value)
const senderInput = typedFilter('sender', () => sender.value)

const params = computed<AttachmentListParams>(() => ({
  q: q.value || undefined,
  sender: sender.value || undefined,
  kind: kind.value,
  from: from.value || undefined,
  to: to.value || undefined,
  sort: sort.value,
  order: order.value,
  limit: limit.value,
  offset: offset.value,
}))

let latest = 0
async function load(quiet = false) {
  const ticket = ++latest
  if (!quiet) loading.value = true
  try {
    const r = await api.listAttachments(params.value)
    if (ticket !== latest) return // a newer query is already on its way
    items.value = r.items
    total.value = r.total
    totalBytes.value = r.total_bytes
    kindCounts.value = r.kinds
  } catch (e) {
    if (!quiet) toast.error(t('attachments.load_error'), { description: String(e) })
  } finally {
    if (ticket === latest) loading.value = false
  }
}
watch(params, () => load(), { deep: true })
onMounted(() => load())

// New mail may bring new attachments; refresh in place without a spinner.
const refreshSoon = useDebounceFn(() => load(true), 400)
const stopMailSignal = onMailSignal(() => void refreshSoon())
onUnmounted(stopMailSignal)

function setSort(col: SortCol) {
  if (sort.value === col) pushQuery({ sort: col, order: order.value === 'asc' ? 'desc' : 'asc', offset: undefined })
  // Names read naturally A→Z; everything else starts biggest/newest first.
  else pushQuery({ sort: col, order: col === 'name' || col === 'type' || col === 'sender' ? 'asc' : 'desc', offset: undefined })
}
function setKind(v: unknown) {
  const k = String(v ?? '')
  pushQuery({ kind: k === ALL_KINDS ? undefined : k, offset: undefined })
}

const hasActiveFilters = computed(() => !!(q.value || sender.value || kind.value || from.value || to.value))
function resetFilters() {
  nameInput.value = ''
  senderInput.value = ''
  pushQuery({ q: undefined, sender: undefined, kind: undefined, from: undefined, to: undefined, offset: undefined })
}

const pageInfo = computed(() => pageLabel(items.value.length, total.value))
const kindTotal = computed(() => ATTACHMENT_KINDS.reduce((sum, k) => sum + (kindCounts.value[k] ?? 0), 0))

const KIND_ICONS: Record<AttachmentKind, unknown> = {
  image: FileImage,
  pdf: FileType,
  document: FileText,
  spreadsheet: FileSpreadsheet,
  archive: FileArchive,
  media: FilePlay,
  other: File,
}

function displayName(a: AttachmentItem) {
  return a.filename || t('viewer.attachment_fallback', { index: a.index })
}

function isoDate(secs: number) {
  return secs > 0 ? new Date(secs * 1000).toISOString() : ''
}

// Mirrors the server's inlineSafe: only these open in a browser tab. The
// server enforces it anyway; this decides whether to offer "Open" at all.
function canOpen(a: AttachmentItem) {
  const ct = a.content_type
  if (ct.startsWith('image/')) return !ct.includes('svg')
  return ct.startsWith('audio/') || ct.startsWith('video/') || ct === 'application/pdf' || ct === 'text/plain'
}

function openURL(a: AttachmentItem) {
  return api.attachmentURL(a.email_sha256, a.index, true)
}
function downloadURL(a: AttachmentItem) {
  return api.attachmentURL(a.email_sha256, a.index)
}
function emailRoute(a: AttachmentItem) {
  return { name: 'viewer', params: { sha: a.email_sha256 }, query: { tab: 'attachments', att: String(a.index) } }
}
const colCount = computed(() => 4 + Number(atLeastMd.value) + Number(atLeastLg.value) + Number(atLeastXl.value))
</script>

<template>
  <section class="flex-1 min-w-0">
    <div class="mb-4 flex flex-wrap items-center gap-3">
      <h1 class="text-2xl font-semibold tracking-tight">{{ t('attachments.title') }}</h1>
      <p v-if="total > 0" class="text-sm text-muted-foreground tabular-nums">
        {{ t('attachments.summary', { count: n(total), size: formatBytes(totalBytes) }, total) }}
      </p>
      <div class="ml-auto flex items-center gap-3">
        <label class="flex items-center gap-2 text-sm text-muted-foreground">
          <span>{{ t('library.per_page') }}</span>
          <Select :model-value="String(limit)" @update:model-value="setPageSize">
            <SelectTrigger class="w-24 h-8">
              <SelectValue>{{ limit }}</SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="size in PAGE_SIZES" :key="size" :value="String(size)">{{ size }}</SelectItem>
            </SelectContent>
          </Select>
        </label>
        <PageNav
          :info="pageInfo"
          :can-prev="offset > 0"
          :can-next="offset + limit < total"
          @prev="goPrevPage"
          @next="goNextPage"
        />
      </div>
    </div>

    <div class="flex items-center flex-wrap gap-3 mb-4">
      <div class="relative w-full max-w-64">
        <Input
          v-model="nameInput"
          type="search"
          :placeholder="t('attachments.search_placeholder')"
          :aria-label="t('attachments.search_placeholder')"
          class="w-full pr-8 [&::-webkit-search-cancel-button]:appearance-none"
        />
        <button
          v-if="nameInput"
          type="button"
          :aria-label="t('library.clear_search')"
          :title="t('library.clear_search')"
          class="absolute right-2 top-1/2 inline-flex h-5 w-5 -translate-y-1/2 items-center justify-center rounded-sm
            text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          @click="nameInput = ''"
        >
          <X class="h-4 w-4" />
        </button>
      </div>
      <div class="w-full max-w-48">
        <Input
          v-model="senderInput"
          type="search"
          :placeholder="t('attachments.sender_placeholder')"
          :aria-label="t('attachments.sender_placeholder')"
          class="w-full [&::-webkit-search-cancel-button]:appearance-none"
        />
      </div>
      <Select :model-value="kind ?? ALL_KINDS" @update:model-value="setKind">
        <SelectTrigger class="h-8 w-52" :aria-label="t('attachments.kind_label')">
          <SelectValue>{{ kind ? t(`attachments.kind.${kind}`) : t('attachments.kind_all') }}</SelectValue>
        </SelectTrigger>
        <SelectContent>
          <SelectItem :value="ALL_KINDS">
            <span class="flex items-center gap-2">
              <span>{{ t('attachments.kind_all') }}</span>
              <span class="tabular-nums text-muted-foreground">{{ n(kindTotal) }}</span>
            </span>
          </SelectItem>
          <SelectItem v-for="k in ATTACHMENT_KINDS" :key="k" :value="k">
            <span class="flex items-center gap-2">
              <span class="flex items-center gap-2">
                <component :is="KIND_ICONS[k]" class="h-4 w-4 text-muted-foreground" />
                {{ t(`attachments.kind.${k}`) }}
              </span>
              <span class="tabular-nums text-muted-foreground">{{ n(kindCounts[k] ?? 0) }}</span>
            </span>
          </SelectItem>
        </SelectContent>
      </Select>
      <DateRangePicker :from="from" :to="to" @change="setDateRange" />
      <Button variant="outline" size="sm" :title="t('library.reset')" :disabled="!hasActiveFilters" @click="resetFilters">
        <RotateCcw class="h-4 w-4" />
        <span class="ml-1.5">{{ t('library.reset') }}</span>
      </Button>
    </div>

    <Card class="overflow-hidden" :aria-busy="loading">
      <table class="w-full text-sm table-fixed">
        <caption class="sr-only">{{ t('attachments.table_caption') }}</caption>
        <!-- A narrower window drops columns rather than squeezing the file
             name to nothing: the email's subject first (the envelope button
             still jumps to it), then the type (the icon shows it), then the
             size. The name takes whatever width is left. -->
        <colgroup>
          <col :class="atLeastXl ? 'w-[30%]' : ''" />
          <col v-if="atLeastLg" class="w-36" />
          <col v-if="atLeastMd" class="w-24" />
          <col class="w-52" />
          <col class="w-36" />
          <col v-if="atLeastXl" />
          <col class="w-28" />
        </colgroup>
        <thead class="bg-muted/40 text-xs uppercase text-muted-foreground">
          <tr class="h-10">
            <SortHeader :label="t('attachments.col.name')" col="name" :sort="sort" :order="order" @sort="setSort" />
            <SortHeader v-if="atLeastLg" :label="t('attachments.col.type')" col="type" :sort="sort" :order="order" @sort="setSort" />
            <SortHeader v-if="atLeastMd" :label="t('attachments.col.size')" col="size" :sort="sort" :order="order" align="right" @sort="setSort" />
            <SortHeader :label="t('attachments.col.date')" col="date" :sort="sort" :order="order" @sort="setSort" />
            <SortHeader :label="t('attachments.col.sender')" col="sender" :sort="sort" :order="order" @sort="setSort" />
            <th v-if="atLeastXl" scope="col" class="px-3 py-2 text-left">{{ t('attachments.col.email') }}</th>
            <th scope="col" class="px-3 py-2"><span class="sr-only">{{ t('attachments.col.actions') }}</span></th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading && items.length === 0">
            <td :colspan="colCount" class="px-3 py-6 text-center text-muted-foreground">{{ t('library.loading') }}</td>
          </tr>
          <tr v-else-if="items.length === 0">
            <td :colspan="colCount" class="px-3 py-6 text-center text-muted-foreground">
              <template v-if="hasActiveFilters">{{ t('attachments.no_matches') }}</template>
              <template v-else>{{ t('attachments.empty') }}</template>
            </td>
          </tr>
          <tr
            v-for="a in items"
            :key="`${a.email_sha256}/${a.index}`"
            class="border-t hover:bg-accent/50"
          >
            <td class="px-3 py-2">
              <span class="flex min-w-0 items-center gap-2">
                <component :is="KIND_ICONS[a.kind]" class="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden="true" />
                <RouterLink
                  v-if="docPreviewKind(a.filename, a.content_type, a.size)"
                  :to="emailRoute(a)"
                  :title="displayName(a)"
                  class="truncate rounded-xs font-medium hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >{{ displayName(a) }}</RouterLink>
                <a
                  v-else
                  :href="canOpen(a) ? openURL(a) : downloadURL(a)"
                  :target="canOpen(a) ? '_blank' : undefined"
                  rel="noopener"
                  :title="displayName(a)"
                  class="truncate rounded-xs font-medium hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >{{ displayName(a) }}</a>
              </span>
            </td>
            <td v-if="atLeastLg" class="px-3 py-2 truncate text-muted-foreground" :title="a.content_type">
              {{ t(`attachments.kind.${a.kind}`) }}
            </td>
            <td v-if="atLeastMd" class="px-3 py-2 text-right whitespace-nowrap tabular-nums text-muted-foreground">{{ formatBytes(a.size) }}</td>
            <td class="px-3 py-2 whitespace-nowrap text-muted-foreground">
              <time
                :datetime="isoDate(a.sent_at) || undefined"
                :title="dateFormat === 'relative' ? formatDateAbsolute(isoDate(a.sent_at)) : undefined"
              >{{ formatDate(isoDate(a.sent_at)) }}</time>
            </td>
            <td class="px-3 py-2 truncate" :title="a.email_from">{{ senderName(a.email_from) }}</td>
            <td v-if="atLeastXl" class="px-3 py-2 truncate">
              <RouterLink
                :to="emailRoute(a)"
                :title="a.email_subject || t('library.no_subject')"
                class="rounded-xs text-muted-foreground hover:text-foreground hover:underline
                  focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >{{ a.email_subject || t('library.no_subject') }}</RouterLink>
            </td>
            <td class="px-3 py-2">
              <div class="flex items-center justify-end gap-1">
                <a
                  v-if="canOpen(a)"
                  :href="openURL(a)"
                  target="_blank"
                  rel="noopener"
                  :title="t('attachments.open')"
                  :aria-label="t('attachments.open_named', { name: displayName(a) })"
                  class="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted-foreground
                    hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <ExternalLink class="h-4 w-4" />
                </a>
                <span v-else class="h-7 w-7" aria-hidden="true"></span>
                <a
                  :href="downloadURL(a)"
                  :title="t('viewer.download')"
                  :aria-label="t('attachments.download_named', { name: displayName(a) })"
                  class="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted-foreground
                    hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <Download class="h-4 w-4" />
                </a>
                <RouterLink
                  :to="emailRoute(a)"
                  :title="t('attachments.go_to_email')"
                  :aria-label="t('attachments.go_to_email')"
                  class="inline-flex h-7 w-7 items-center justify-center rounded-sm text-muted-foreground
                    hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                >
                  <Mail class="h-4 w-4" />
                </RouterLink>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </Card>

    <div v-if="items.length" class="mt-4 flex justify-end">
      <PageNav
        :info="pageInfo"
        :can-prev="offset > 0"
        :can-next="offset + limit < total"
        @prev="goPrevPage"
        @next="goNextPage"
      />
    </div>
  </section>
</template>
