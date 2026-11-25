<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useElementSize } from '@vueuse/core'
import type { Stats } from '@/lib/api'
import { formatBytes } from '@/lib/format'
import ChartTooltip from './ChartTooltip.vue'
import { compact, niceTicks } from './scale'

// Mail volume over time: an area per month, or columns per year.
const props = defineProps<{
  perMonth: Stats['per_month']
  perYear: Stats['per_year']
  mode: 'month' | 'year'
}>()

const { t, locale } = useI18n()

const HEIGHT = 220
const PAD = { top: 12, right: 8, bottom: 24, left: 40 }

const root = ref<HTMLElement | null>(null)
const { width } = useElementSize(root)

interface Point {
  key: string
  label: string
  count: number
  bytes?: number
  /** Axis label, only on points that get one. */
  tick?: string
}

// Every month from the first to the last, zeros included, so a gap in the
// archive shows as a dip rather than being skipped over.
const monthPoints = computed<Point[]>(() => {
  const ms = props.perMonth
  if (!ms.length) return []
  const byKey = new Map(ms.map((m) => [m.month, m]))
  const [y0, m0] = ms[0]!.month.split('-').map(Number) as [number, number]
  const [y1, m1] = ms[ms.length - 1]!.month.split('-').map(Number) as [number, number]
  const fmt = new Intl.DateTimeFormat(locale.value, { year: 'numeric', month: 'long' })
  const out: Point[] = []
  for (let y = y0, m = m0; y < y1 || (y === y1 && m <= m1); ) {
    const key = `${y}-${String(m).padStart(2, '0')}`
    const hit = byKey.get(key)
    out.push({
      key,
      label: fmt.format(new Date(y, m - 1, 1)),
      count: hit?.count ?? 0,
      bytes: hit?.bytes ?? 0,
      tick: m === 1 || out.length === 0 ? String(y) : undefined,
    })
    if (++m > 12) {
      m = 1
      y++
    }
  }
  return out
})

const yearPoints = computed<Point[]>(() => {
  const ys = props.perYear
  if (!ys.length) return []
  const byYear = new Map(ys.map((y) => [Number(y.year), y.count]))
  const first = Number(ys[0]!.year)
  const last = Number(ys[ys.length - 1]!.year)
  const out: Point[] = []
  for (let y = first; y <= last; y++) {
    out.push({ key: String(y), label: String(y), count: byYear.get(y) ?? 0, tick: String(y) })
  }
  return out
})

// A single month can't draw an area; show it as a column instead.
const asColumns = computed(() => props.mode === 'year' || monthPoints.value.length < 2)
const points = computed(() => (props.mode === 'year' ? yearPoints.value : monthPoints.value))

const plotW = computed(() => Math.max(0, width.value - PAD.left - PAD.right))
const plotH = HEIGHT - PAD.top - PAD.bottom

const ticks = computed(() => niceTicks(Math.max(1, ...points.value.map((p) => p.count))))
const yMax = computed(() => ticks.value[ticks.value.length - 1]!)
const y = (v: number) => PAD.top + plotH - (v / yMax.value) * plotH

// Columns: one band per point. Area: points spread edge to edge.
const band = computed(() => plotW.value / Math.max(1, points.value.length))
const x = (i: number) =>
  asColumns.value
    ? PAD.left + band.value * (i + 0.5)
    : PAD.left + (points.value.length < 2 ? 0 : (i / (points.value.length - 1)) * plotW.value)

// Thin labels out so they never collide: about one per 56 px.
const xTicks = computed(() => {
  const labelled = points.value.map((p, i) => ({ i, text: p.tick })).filter((p) => p.text)
  const every = Math.max(1, Math.ceil((labelled.length * 56) / Math.max(1, plotW.value)))
  return labelled.filter((_, k) => k % every === 0)
})

const linePath = computed(() =>
  points.value.map((p, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(p.count).toFixed(1)}`).join(''),
)
const areaPath = computed(() => {
  const n = points.value.length
  if (n < 2) return ''
  const base = y(0).toFixed(1)
  return `${linePath.value}L${x(n - 1).toFixed(1)},${base}L${x(0).toFixed(1)},${base}Z`
})

// Column with a 4px rounded top, square at the baseline; 2px gap between.
function columnPath(i: number, v: number): string {
  const w = Math.max(1, band.value - 2)
  const left = x(i) - w / 2
  const top = y(v)
  const base = y(0)
  const h = base - top
  if (h <= 0) return ''
  const r = Math.min(4, w / 2, h)
  return `M${left},${base}V${top + r}Q${left},${top} ${left + r},${top}H${left + w - r}Q${left + w},${top} ${left + w},${top + r}V${base}Z`
}

const hover = ref<number | null>(null)
const hoverPoint = computed(() => (hover.value === null ? null : points.value[hover.value]))

function onMove(e: PointerEvent) {
  const rect = root.value!.getBoundingClientRect()
  const px = e.clientX - rect.left
  const n = points.value.length
  if (!n) return
  const i = asColumns.value
    ? Math.floor((px - PAD.left) / band.value)
    : Math.round(((px - PAD.left) / Math.max(1, plotW.value)) * (n - 1))
  hover.value = Math.min(n - 1, Math.max(0, i))
}

const n = (v: number) => v.toLocaleString(locale.value)
</script>

<template>
  <div ref="root" class="relative select-none" @pointermove="onMove" @pointerleave="hover = null">
    <svg v-if="width > 0" :width="width" :height="HEIGHT" class="block overflow-visible" role="img" :aria-label="t('stats.volume')">
      <g class="text-[11px]" fill="var(--muted-foreground)">
        <template v-for="tk in ticks" :key="tk">
          <line :x1="PAD.left" :x2="width - PAD.right" :y1="y(tk)" :y2="y(tk)" stroke="var(--chart-grid)" stroke-width="1" />
          <text :x="PAD.left - 8" :y="y(tk)" text-anchor="end" dominant-baseline="middle" class="tabular-nums">{{ compact(tk, locale) }}</text>
        </template>
        <text
          v-for="tk in xTicks"
          :key="tk.i"
          :x="x(tk.i)"
          :y="HEIGHT - 6"
          :text-anchor="asColumns ? 'middle' : tk.i === 0 ? 'start' : 'middle'"
          class="tabular-nums"
        >{{ tk.text }}</text>
      </g>

      <template v-if="asColumns">
        <path
          v-for="(p, i) in points"
          :key="p.key"
          :d="columnPath(i, p.count)"
          fill="var(--primary)"
          :opacity="hover === null || hover === i ? 1 : 0.55"
        />
      </template>
      <template v-else>
        <path :d="areaPath" fill="var(--primary)" opacity="0.14" />
        <path :d="linePath" fill="none" stroke="var(--primary)" stroke-width="2" stroke-linejoin="round" />
        <template v-if="hoverPoint">
          <line :x1="x(hover!)" :x2="x(hover!)" :y1="PAD.top" :y2="y(0)" stroke="var(--muted-foreground)" stroke-width="1" stroke-dasharray="2 3" />
          <circle :cx="x(hover!)" :cy="y(hoverPoint.count)" r="4" fill="var(--primary)" stroke="var(--card)" stroke-width="2" />
        </template>
      </template>
    </svg>

    <ChartTooltip v-if="hoverPoint" :x="x(hover!)" :y="y(hoverPoint.count)" :container-width="width">
      <div class="font-medium">{{ hoverPoint.label }}</div>
      <div class="tabular-nums text-muted-foreground">
        {{ t('stats.emails_count', { count: n(hoverPoint.count) }, hoverPoint.count) }}<template v-if="hoverPoint.bytes"> · {{ formatBytes(hoverPoint.bytes) }}</template>
      </div>
    </ChartTooltip>
  </div>
</template>
