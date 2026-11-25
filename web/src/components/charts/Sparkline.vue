<script setup lang="ts">
import { computed, ref } from 'vue'
import { useElementSize } from '@vueuse/core'

// A small trend line for a value that refreshes live. Fixed slot count so
// the line scrolls left as samples arrive instead of stretching.
const props = withDefaults(defineProps<{ values: number[]; slots: number; max?: number; height?: number }>(), {
  max: undefined,
  height: 36,
})

const root = ref<HTMLElement | null>(null)
const { width } = useElementSize(root)

const top = computed(() => props.max ?? Math.max(1, ...props.values) * 1.15)
const pts = computed(() => {
  const w = width.value
  const h = props.height - 3
  const step = w / Math.max(1, props.slots - 1)
  const offset = props.slots - props.values.length
  return props.values.map((v, i) => [(offset + i) * step, 2 + h - (Math.min(v, top.value) / top.value) * (h - 2)] as const)
})
const line = computed(() => pts.value.map(([x, y], i) => `${i ? 'L' : 'M'}${x.toFixed(1)},${y.toFixed(1)}`).join(''))
const area = computed(() => {
  const p = pts.value
  if (p.length < 2) return ''
  return `${line.value}L${p[p.length - 1]![0].toFixed(1)},${props.height}L${p[0]![0].toFixed(1)},${props.height}Z`
})
const last = computed(() => pts.value[pts.value.length - 1])
</script>

<template>
  <div ref="root" class="w-full">
    <svg v-if="width > 0" :width="width" :height="height" class="block overflow-visible" aria-hidden="true">
      <line x1="0" :x2="width" :y1="height - 0.5" :y2="height - 0.5" stroke="var(--chart-grid)" />
      <path :d="area" fill="var(--primary)" opacity="0.14" />
      <path :d="line" fill="none" stroke="var(--primary)" stroke-width="2" stroke-linejoin="round" />
      <circle v-if="last" :cx="last[0]" :cy="last[1]" r="3" fill="var(--primary)" stroke="var(--card)" stroke-width="1.5" />
    </svg>
  </div>
</template>
