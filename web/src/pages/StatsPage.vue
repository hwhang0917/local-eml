<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDocumentVisibility, useIntervalFn, useStorage } from '@vueuse/core'
import {
  AlertTriangle,
  Ban,
  FolderX,
  Mail,
  ShieldAlert,
  Star,
} from 'lucide-vue-next'
import { api, type AppStats, type AttachmentKind, type Stats } from '@/lib/api'
import { formatBytes, senderName } from '@/lib/format'
import { useCategories } from '@/composables/useCategories'
import { useReadTracking } from '@/composables/useReadTracking'
import Card from '@/components/ui/Card.vue'
import CategoryDot from '@/components/ui/CategoryDot.vue'
import VolumeChart from '@/components/charts/VolumeChart.vue'
import ActivityHeatmap from '@/components/charts/ActivityHeatmap.vue'
import BarList, { type BarRow } from '@/components/charts/BarList.vue'
import StackedBar, { type Segment } from '@/components/charts/StackedBar.vue'
import Sparkline from '@/components/charts/Sparkline.vue'

const { t, locale } = useI18n()
const n = (v: number) => v.toLocaleString(locale.value)
const { byId, labelFor, load: loadCategories } = useCategories()
const { enabled: readTracking } = useReadTracking()

const stats = ref<Stats | null>(null)
const error = ref('')

onMounted(async () => {
  try {
    ;[stats.value] = await Promise.all([api.getStats(), loadCategories()])
  } catch (e) {
    error.value = String(e)
  }
})

const tiles = computed(() => {
  const s = stats.value
  if (!s) return []
  return [
    { label: t('stats.total'), value: n(s.total_count) },
    { label: t('stats.total_size'), value: formatBytes(s.total_bytes) },
    { label: t('stats.starred'), value: n(s.starred_count) },
    { label: t('stats.with_attachments'), value: n(s.attachment_count) },
  ]
})

// --- Inbox health -----------------------------------------------------------

const health = computed(() => {
  const s = stats.value
  if (!s) return []
  const pct = (v: number) => (s.total_count ? Math.round((v / s.total_count) * 100) : 0)
  return [
    readTracking.value && {
      key: 'unread', icon: Mail, value: s.unread_count, pct: pct(s.unread_count),
      to: { name: 'library', query: { q: 'is:unread' } },
    },
    {
      key: 'starred', icon: Star, value: s.starred_count, pct: pct(s.starred_count),
      to: { name: 'library', query: { q: 'is:starred' } },
    },
    {
      key: 'uncategorized', icon: FolderX, value: s.uncategorized_count, pct: pct(s.uncategorized_count),
      to: { name: 'library', query: { q: 'category:none' } },
    },
    { key: 'spam', icon: Ban, value: s.spam_count, pct: undefined, to: { name: 'settings-flagged' } },
    { key: 'phishing', icon: ShieldAlert, value: s.phishing_count, pct: undefined, to: { name: 'settings-flagged' } },
    { key: 'risky', icon: AlertTriangle, value: s.risky_count, pct: undefined, to: undefined },
  ].filter((h) => !!h)
})

// --- Volume over time -------------------------------------------------------

const volumeMode = useStorage<'month' | 'year'>('stats-volume-mode', 'month')

// --- Senders / domains ------------------------------------------------------

const senderMode = useStorage<'sender' | 'domain'>('stats-sender-mode', 'sender')

const senderRows = computed<BarRow[]>(() => {
  const s = stats.value
  if (!s) return []
  if (senderMode.value === 'domain') {
    return s.top_domains.map((d) => ({
      key: d.domain,
      label: d.domain,
      value: d.count,
      to: { name: 'library', query: { q: `from:@${d.domain}` } },
    }))
  }
  return s.top_senders.map((x) => ({
    key: x.address,
    label: x.name || x.address,
    title: x.name ? `${x.name} <${x.address}>` : x.address,
    value: x.count,
    to: { name: 'library', query: { q: `from:${x.address}` } },
  }))
})

// --- Storage ----------------------------------------------------------------

// Each kind owns a series slot (see --series-* in style.css); "other" and
// the message text are neutral, so they never read as a seventh hue.
const KIND_COLOR: Record<AttachmentKind, string> = {
  image: 'var(--series-1)',
  pdf: 'var(--series-2)',
  document: 'var(--series-3)',
  spreadsheet: 'var(--series-4)',
  archive: 'var(--series-5)',
  media: 'var(--series-6)',
  other: 'var(--series-neutral)',
}
const KIND_ORDER: AttachmentKind[] = ['image', 'pdf', 'document', 'spreadsheet', 'archive', 'media', 'other']

// Attachments are stored decoded-size in the index but base64 inside the
// .eml, about 4/3 as big; scale them so the parts add up to the real total.
const BASE64 = 4 / 3

const storage = computed(() => {
  const s = stats.value
  if (!s) return null
  const byKind = new Map(s.per_kind.map((k) => [k.kind, k]))
  const attachments = KIND_ORDER.flatMap((kind) => {
    const k = byKind.get(kind)
    if (!k) return []
    return [{
      key: kind,
      label: t(`attachments.kind.${kind}`),
      value: Math.min(k.bytes * BASE64, s.total_bytes),
      bytes: k.bytes,
      count: k.count,
      color: KIND_COLOR[kind],
    }]
  })
  const attachmentTotal = attachments.reduce((a, k) => a + k.value, 0)
  const text = Math.max(0, s.total_bytes - attachmentTotal)
  const segments: Segment[] = [
    { key: 'text', label: t('stats.storage_text'), value: text, color: 'var(--muted-foreground)', detail: formatBytes(text) },
    ...attachments.map((k) => ({ ...k, detail: formatBytes(k.bytes) })),
  ]
  return {
    segments,
    legend: [...attachments].sort((a, b) => b.bytes - a.bytes),
    text,
  }
})

// --- Categories -------------------------------------------------------------

const categoryRows = computed(() =>
  (stats.value?.per_category ?? [])
    .map((c) => ({ ...c, category: byId.value.get(c.category_id) }))
    .filter((c) => c.category)
    .sort((a, b) => b.count - a.count),
)

// --- App resources (live) ---------------------------------------------------

const POLL_MS = 3000
const SPARK_SLOTS = 40

const app = ref<AppStats | null>(null)
const cpuHistory = ref<number[]>([])
const memHistory = ref<number[]>([])

async function pollApp() {
  try {
    const a = await api.getAppStats()
    // The first reading covers everything since start, not the last few
    // seconds; keep the number but leave it out of the trend line.
    const first = app.value === null
    app.value = a
    if (!first) cpuHistory.value = [...cpuHistory.value, a.cpu_percent].slice(-SPARK_SLOTS)
    memHistory.value = [...memHistory.value, a.memory_bytes].slice(-SPARK_SLOTS)
  } catch {
    // A missed sample just leaves a gap; the offline banner covers a dead server.
  }
}

// Only poll while the tab is visible: a hidden stats tab shouldn't keep the
// server (or a laptop's CPU) busy.
const visibility = useDocumentVisibility()
const { resume } = useIntervalFn(() => {
  if (visibility.value === 'visible') void pollApp()
}, POLL_MS, { immediate: false })
onMounted(() => {
  void pollApp()
  resume()
})

const disk = computed(() => {
  const d = app.value?.disk
  if (!d || !d.total_bytes) return null
  const library = d.db_bytes + d.eml_bytes + d.logs_bytes
  const otherUsed = Math.max(0, d.total_bytes - d.free_bytes - library)
  const segments: Segment[] = [
    { key: 'eml', label: t('stats.app.disk_eml'), value: d.eml_bytes, color: 'var(--series-1)', detail: formatBytes(d.eml_bytes) },
    { key: 'db', label: t('stats.app.disk_db'), value: d.db_bytes, color: 'var(--series-2)', detail: formatBytes(d.db_bytes) },
    { key: 'logs', label: t('stats.app.disk_logs'), value: d.logs_bytes, color: 'var(--series-3)', detail: formatBytes(d.logs_bytes) },
    { key: 'other', label: t('stats.app.disk_other'), value: otherUsed, color: 'var(--series-neutral)', detail: formatBytes(otherUsed) },
  ]
  return { d, library, segments, usedPct: Math.round(((d.total_bytes - d.free_bytes) / d.total_bytes) * 100) }
})

function formatUptime(sec: number): string {
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  if (d) return t('stats.app.uptime_dh', { d, h })
  if (h) return t('stats.app.uptime_hm', { h, m })
  return t('stats.app.uptime_m', { m })
}

const cpuLabel = computed(() => {
  const c = app.value?.cpu_percent ?? 0
  return `${c < 0.1 && c > 0 ? '<0.1' : c.toFixed(1)}%`
})
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold tracking-tight">{{ t('stats.title') }}</h1>

    <p v-if="error" class="text-sm text-destructive">{{ error }}</p>
    <p v-else-if="stats && stats.total_count === 0" class="text-sm text-muted-foreground">
      {{ t('stats.empty') }}
    </p>

    <template v-else-if="stats">
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Card v-for="tile in tiles" :key="tile.label" class="p-4">
          <div class="text-sm text-muted-foreground">{{ tile.label }}</div>
          <div class="mt-1 text-2xl font-semibold tabular-nums">{{ tile.value }}</div>
        </Card>
      </div>

      <!-- Volume over time -->
      <Card class="p-6">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-base font-semibold">{{ t('stats.volume') }}</h2>
          <div class="inline-flex rounded-md border border-hairline p-0.5 text-xs" role="group">
            <button
              v-for="m in (['month', 'year'] as const)"
              :key="m"
              type="button"
              class="rounded px-2.5 py-1"
              :class="volumeMode === m ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'"
              :aria-pressed="volumeMode === m"
              @click="volumeMode = m"
            >{{ t(`stats.by_${m}`) }}</button>
          </div>
        </div>
        <VolumeChart class="mt-4" :per-month="stats.per_month" :per-year="stats.per_year" :mode="volumeMode" />
        <p v-if="stats.undated_count > 0" class="mt-3 text-xs text-muted-foreground">
          {{ t('stats.undated', { count: n(stats.undated_count) }) }}
        </p>
      </Card>

      <div class="grid gap-6 lg:grid-cols-5 lg:items-start">
        <!-- When mail arrives -->
        <Card class="p-6 lg:col-span-3">
          <h2 class="text-base font-semibold">{{ t('stats.heatmap') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('stats.heatmap_hint') }}</p>
          <ActivityHeatmap class="mt-4" :heatmap="stats.heatmap" />
        </Card>

        <!-- Top senders / domains -->
        <Card class="p-6 lg:col-span-2">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 class="text-base font-semibold">{{ t(senderMode === 'sender' ? 'stats.top_senders' : 'stats.top_domains') }}</h2>
            <div class="inline-flex rounded-md border border-hairline p-0.5 text-xs" role="group">
              <button
                v-for="m in (['sender', 'domain'] as const)"
                :key="m"
                type="button"
                class="rounded px-2.5 py-1"
                :class="senderMode === m ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:text-foreground'"
                :aria-pressed="senderMode === m"
                @click="senderMode = m"
              >{{ t(`stats.by_${m}`) }}</button>
            </div>
          </div>
          <BarList class="mt-3" :rows="senderRows" />
        </Card>
      </div>

      <!-- Inbox health -->
      <Card class="p-6">
        <h2 class="text-base font-semibold">{{ t('stats.health') }}</h2>
        <div class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
          <component
            :is="h.to ? 'RouterLink' : 'div'"
            v-for="h in health"
            :key="h.key"
            :to="h.to"
            class="rounded-md border border-hairline p-3"
            :class="h.to ? 'hover:bg-accent focus-visible:bg-accent focus-visible:outline-none' : ''"
          >
            <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
              <component :is="h.icon" class="size-3.5" aria-hidden="true" />
              {{ t(`stats.health_${h.key}`) }}
            </div>
            <div class="mt-1 flex items-baseline gap-1.5">
              <span class="text-xl font-semibold tabular-nums">{{ n(h.value) }}</span>
              <span v-if="h.pct !== undefined" class="text-xs tabular-nums text-muted-foreground">{{ h.pct }}%</span>
            </div>
          </component>
        </div>
      </Card>

      <div class="grid gap-6 lg:grid-cols-2">
        <!-- Storage -->
        <Card v-if="storage" class="p-6">
          <h2 class="text-base font-semibold">{{ t('stats.storage') }}</h2>
          <p class="mt-1 text-xs text-muted-foreground">{{ t('stats.storage_hint', { size: formatBytes(stats.total_bytes) }) }}</p>
          <StackedBar class="mt-4" :segments="storage.segments" />
          <ul class="mt-4 space-y-1 text-sm">
            <li class="flex items-center gap-2 py-0.5">
              <span class="size-2.5 shrink-0 rounded-sm bg-muted-foreground" aria-hidden="true"></span>
              <span class="min-w-0 flex-1 truncate">{{ t('stats.storage_text') }}</span>
              <span class="shrink-0 tabular-nums text-muted-foreground">{{ formatBytes(storage.text) }}</span>
            </li>
            <li v-for="k in storage.legend" :key="k.key">
              <RouterLink
                :to="{ name: 'attachments', query: { kind: k.key, sort: 'size' } }"
                class="-mx-2 flex items-center gap-2 rounded-md px-2 py-0.5 hover:bg-accent"
              >
                <span class="size-2.5 shrink-0 rounded-sm" :style="{ background: k.color }" aria-hidden="true"></span>
                <span class="min-w-0 flex-1 truncate">{{ k.label }}</span>
                <span class="shrink-0 tabular-nums text-muted-foreground">{{ t('stats.files', { count: n(k.count) }, k.count) }}</span>
                <span class="w-20 shrink-0 text-right tabular-nums text-muted-foreground">{{ formatBytes(k.bytes) }}</span>
              </RouterLink>
            </li>
          </ul>
        </Card>

        <!-- Largest emails -->
        <Card class="p-6">
          <h2 class="text-base font-semibold">{{ t('stats.largest') }}</h2>
          <ol class="mt-3 space-y-1">
            <li v-for="e in stats.largest" :key="e.sha256">
              <RouterLink
                :to="{ name: 'viewer', params: { sha: e.sha256 } }"
                class="-mx-2 flex items-baseline gap-3 rounded-md px-2 py-1.5 text-sm hover:bg-accent"
              >
                <span class="min-w-0 flex-1">
                  <span class="block truncate">{{ e.subject || t('stats.no_subject') }}</span>
                  <span class="block truncate text-xs text-muted-foreground">
                    {{ senderName(e.from) || e.from }}<template v-if="e.sent_at"> · {{ new Date(e.sent_at * 1000).toLocaleDateString(locale) }}</template>
                  </span>
                </span>
                <span class="shrink-0 tabular-nums text-muted-foreground">{{ formatBytes(e.size_bytes) }}</span>
              </RouterLink>
            </li>
          </ol>
        </Card>
      </div>

      <Card v-if="categoryRows.length" class="p-6">
        <h2 class="text-base font-semibold">{{ t('stats.per_category') }}</h2>
        <ul class="mt-4 grid gap-x-8 gap-y-2 sm:grid-cols-2">
          <li
            v-for="c in categoryRows"
            :key="c.category_id"
          >
            <RouterLink
              :to="{ name: 'library', query: { q: `category:${c.category!.color}` } }"
              class="-mx-2 flex items-center gap-2 rounded-md px-2 py-0.5 text-sm hover:bg-accent"
            >
              <CategoryDot :color="c.category!.color" />
              <span class="min-w-0 flex-1 truncate">{{ labelFor(c.category) }}</span>
              <span class="shrink-0 tabular-nums text-muted-foreground">{{ n(c.count) }}</span>
            </RouterLink>
          </li>
        </ul>
      </Card>
    </template>

    <!-- The app itself: shown even on an empty library. -->
    <Card v-if="app" class="p-6">
      <div class="flex flex-wrap items-baseline justify-between gap-2">
        <h2 class="text-base font-semibold">{{ t('stats.app.title') }}</h2>
        <span class="text-xs text-muted-foreground">{{ t('stats.app.live') }}</span>
      </div>
      <div class="mt-4 grid gap-6 md:grid-cols-3">
        <div>
          <div class="text-sm text-muted-foreground">{{ t('stats.app.memory') }}</div>
          <div class="mt-1 text-2xl font-semibold tabular-nums">{{ formatBytes(app.memory_bytes) }}</div>
          <Sparkline class="mt-2" :values="memHistory" :slots="SPARK_SLOTS" />
        </div>
        <div>
          <div class="text-sm text-muted-foreground">{{ t('stats.app.cpu') }}</div>
          <div class="mt-1 text-2xl font-semibold tabular-nums">{{ cpuLabel }}</div>
          <Sparkline class="mt-2" :values="cpuHistory" :slots="SPARK_SLOTS" :max="Math.max(5, ...cpuHistory) * 1.15" />
        </div>
        <div v-if="disk">
          <div class="text-sm text-muted-foreground">{{ t('stats.app.library_size') }}</div>
          <div class="mt-1 text-2xl font-semibold tabular-nums">{{ formatBytes(disk.library) }}</div>
          <div class="mt-3 text-xs text-muted-foreground">
            {{ t('stats.app.disk_free', { free: formatBytes(disk.d.free_bytes), total: formatBytes(disk.d.total_bytes) }) }}
          </div>
          <StackedBar class="mt-1.5" :segments="disk.segments" :total="disk.d.total_bytes" :height="10" />
        </div>
      </div>
      <ul v-if="disk" class="mt-4 flex flex-wrap gap-x-5 gap-y-1 text-xs text-muted-foreground">
        <li v-for="s in disk.segments" :key="s.key" class="flex items-center gap-1.5">
          <span class="size-2.5 rounded-sm" :style="{ background: s.color }" aria-hidden="true"></span>
          {{ s.label }} <span class="tabular-nums">{{ s.detail }}</span>
        </li>
        <li class="flex items-center gap-1.5">
          <span class="size-2.5 rounded-sm bg-muted ring-1 ring-hairline ring-inset" aria-hidden="true"></span>
          {{ t('stats.app.disk_free_label') }} <span class="tabular-nums">{{ formatBytes(disk.d.free_bytes) }}</span>
        </li>
      </ul>
      <p class="mt-4 border-t border-hairline pt-3 text-xs text-muted-foreground">
        {{ t('stats.app.footer', { uptime: formatUptime(app.uptime_sec), version: app.version || 'dev', go: app.go_version, cpus: app.num_cpu }) }}
      </p>
    </Card>
  </div>
</template>
