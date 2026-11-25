<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useElementSize } from '@vueuse/core'
import ChartTooltip from './ChartTooltip.vue'

// One horizontal bar split into parts, with a 2px card-coloured gap between
// them. Parts keep the order given, so a part keeps its neighbours (and its
// colour) however the sizes change. total may exceed the sum: the rest is
// left as empty track (free space, say).
export interface Segment {
  key: string
  label: string
  value: number
  color: string
  /** Tooltip second line; defaults to nothing. */
  detail?: string
}

const props = withDefaults(defineProps<{ segments: Segment[]; total?: number; height?: number }>(), {
  total: undefined,
  height: 16,
})

const clipId = `sb-${useId()}`
const root = ref<HTMLElement | null>(null)
const { width } = useElementSize(root)

const sum = computed(() => props.total ?? props.segments.reduce((a, s) => a + s.value, 0))

const laid = computed(() => {
  const w = width.value
  const total = sum.value
  if (!w || total <= 0) return []
  let x = 0
  return props.segments
    .filter((s) => s.value > 0)
    .map((s) => {
      // At least 2px so a tiny part is still visible and hoverable.
      const sw = Math.max(2, (s.value / total) * w)
      const out = { ...s, x, w: sw, pct: (s.value / total) * 100 }
      x += sw
      return out
    })
})

const hover = ref<string | null>(null)
const hovered = computed(() => laid.value.find((s) => s.key === hover.value) ?? null)
</script>

<template>
  <div ref="root" class="relative" @pointerleave="hover = null">
    <svg v-if="width > 0" :width="width" :height="height" class="block" role="img" :aria-label="segments.map((s) => s.label).join(', ')">
      <defs>
        <clipPath :id="clipId">
          <rect :width="width" :height="height" :rx="Math.min(4, height / 2)" />
        </clipPath>
      </defs>
      <g :clip-path="`url(#${clipId})`">
        <rect :width="width" :height="height" fill="var(--muted)" />
        <rect
          v-for="s in laid"
          :key="s.key"
          :x="s.x"
          :width="s.w"
          :height="height"
          :fill="s.color"
          stroke="var(--card)"
          stroke-width="2"
          :opacity="hover === null || hover === s.key ? 1 : 0.6"
          @pointerenter="hover = s.key"
        />
      </g>
    </svg>
    <ChartTooltip v-if="hovered" :x="hovered.x + hovered.w / 2" :y="height" :container-width="width">
      <div class="font-medium">{{ hovered.label }}</div>
      <div class="tabular-nums text-muted-foreground">
        {{ hovered.detail }}<template v-if="hovered.detail"> · </template>{{ hovered.pct < 1 ? '<1' : Math.round(hovered.pct) }}%
      </div>
    </ChartTooltip>
  </div>
</template>
