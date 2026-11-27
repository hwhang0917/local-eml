<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useElementSize } from '@vueuse/core'
import { weekStartsOn } from '@/lib/format'
import ChartTooltip from './ChartTooltip.vue'

// When mail arrives: 7 weekday rows x 24 hour columns, shaded by count.
const props = defineProps<{ heatmap: number[][] }>()

const { t, locale } = useI18n()

const LABEL_W = 36
const AXIS_H = 18
const GAP = 2
// Five shades plus "none". Square-root scale, so one very busy hour doesn't
// wash every other cell out to the lightest shade.
const SHADES = [18, 36, 56, 78, 100]

const root = ref<HTMLElement | null>(null)
const { width } = useElementSize(root)

const cell = computed(() => Math.max(6, Math.min(28, (width.value - LABEL_W) / 24 - GAP)))
const gridW = computed(() => LABEL_W + 24 * (cell.value + GAP))
const height = computed(() => 7 * (cell.value + GAP) + AXIS_H)

const days = computed(() => {
  const start = weekStartsOn.value
  const fmt = new Intl.DateTimeFormat(locale.value, { weekday: 'short' })
  const long = new Intl.DateTimeFormat(locale.value, { weekday: 'long' })
  // 2023-01-01 was a Sunday.
  return Array.from({ length: 7 }, (_, k) => {
    const wd = (start + k) % 7
    const d = new Date(2023, 0, 1 + wd)
    return { wd, short: fmt.format(d), long: long.format(d) }
  })
})

const hourFmt = computed(() => new Intl.DateTimeFormat(locale.value, { hour: 'numeric' }))
const hourLabel = (h: number) => hourFmt.value.format(new Date(2023, 0, 1, h))

const max = computed(() => Math.max(0, ...props.heatmap.flat()))

function shade(v: number): string {
  if (v <= 0 || max.value <= 0) return 'var(--muted)'
  const level = Math.min(SHADES.length - 1, Math.floor(Math.sqrt(v / max.value) * SHADES.length - 1e-9))
  return `color-mix(in oklab, var(--primary) ${SHADES[Math.max(0, level)]}%, var(--muted))`
}

const busiest = computed(() => {
  let best = { wd: 0, h: 0, v: 0 }
  props.heatmap.forEach((row, wd) =>
    row.forEach((v, h) => {
      if (v > best.v) best = { wd, h, v }
    }),
  )
  return best.v > 0 ? best : null
})

const hover = ref<{ row: number; h: number } | null>(null)
const hoverInfo = computed(() => {
  if (!hover.value) return null
  const day = days.value[hover.value.row]!
  const h = hover.value.h
  return {
    day: day.long,
    range: `${hourLabel(h)} – ${hourLabel((h + 1) % 24)}`,
    count: props.heatmap[day.wd]?.[h] ?? 0,
  }
})
const cx = (h: number) => LABEL_W + h * (cell.value + GAP)
const cy = (row: number) => row * (cell.value + GAP)
const n = (v: number) => v.toLocaleString(locale.value)
</script>

<template>
  <div>
    <div ref="root" class="relative select-none" @pointerleave="hover = null">
      <svg v-if="width > 0" :width="gridW" :height="height" class="block" role="img" :aria-label="t('stats.heatmap')">
        <text
          v-for="(d, row) in days"
          :key="d.wd"
          x="0"
          :y="cy(row) + cell / 2"
          dominant-baseline="middle"
          class="text-[0.6875rem]"
          fill="var(--muted-foreground)"
        >{{ d.short }}</text>
        <template v-for="(d, row) in days" :key="`r${d.wd}`">
          <rect
            v-for="h in 24"
            :key="h"
            :x="cx(h - 1)"
            :y="cy(row)"
            :width="cell"
            :height="cell"
            rx="2"
            :fill="shade(heatmap[d.wd]?.[h - 1] ?? 0)"
            :stroke="hover && hover.row === row && hover.h === h - 1 ? 'var(--foreground)' : 'none'"
            stroke-width="1.5"
            @pointerenter="hover = { row, h: h - 1 }"
          />
        </template>
        <text
          v-for="h in [0, 6, 12, 18]"
          :key="`h${h}`"
          :x="cx(h)"
          :y="height - 4"
          class="text-[0.6875rem]"
          fill="var(--muted-foreground)"
        >{{ hourLabel(h) }}</text>
      </svg>
      <ChartTooltip v-if="hover && hoverInfo" :x="cx(hover.h) + cell" :y="cy(hover.row)" :container-width="width">
        <div class="font-medium">{{ hoverInfo.day }} {{ hoverInfo.range }}</div>
        <div class="tabular-nums text-muted-foreground">{{ t('stats.emails_count', { count: n(hoverInfo.count) }, hoverInfo.count) }}</div>
      </ChartTooltip>
    </div>

    <div class="mt-3 flex flex-wrap items-center justify-between gap-3 text-xs text-muted-foreground">
      <span v-if="busiest">
        {{ t('stats.busiest', { day: days.find((d) => d.wd === busiest!.wd)!.long, hour: hourLabel(busiest.h) }) }}
      </span>
      <span class="flex items-center gap-1">
        {{ t('stats.fewer') }}
        <svg :width="6 * 12" height="10" aria-hidden="true">
          <rect v-for="(c, i) in ['var(--muted)', ...SHADES.map((p) => `color-mix(in oklab, var(--primary) ${p}%, var(--muted))`)]" :key="i" :x="i * 12" y="0" width="10" height="10" rx="2" :fill="c" />
        </svg>
        {{ t('stats.more') }}
      </span>
    </div>
  </div>
</template>
