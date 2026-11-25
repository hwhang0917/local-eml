<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'

// Ranked rows: label and value on top, a thin bar scaled to the largest row
// underneath. Every value is printed, so the bars never carry it alone.
export interface BarRow {
  key: string
  label: string
  /** Shown on hover, e.g. the full address behind a display name. */
  title?: string
  value: number
  to?: RouteLocationRaw
}

const props = defineProps<{ rows: BarRow[] }>()
const { locale } = useI18n()

const max = computed(() => Math.max(1, ...props.rows.map((r) => r.value)))
const pct = (v: number) => Math.max(0.5, (v / max.value) * 100)
const n = (v: number) => v.toLocaleString(locale.value)
</script>

<template>
  <ol class="space-y-1">
    <li v-for="r in rows" :key="r.key">
      <component
        :is="r.to ? 'RouterLink' : 'div'"
        :to="r.to"
        :title="r.title ?? r.label"
        class="-mx-2 block rounded-md px-2 py-1.5"
        :class="r.to ? 'hover:bg-accent focus-visible:bg-accent focus-visible:outline-none' : ''"
      >
        <div class="flex items-baseline gap-3 text-sm">
          <span class="min-w-0 flex-1 truncate">{{ r.label }}</span>
          <span class="shrink-0 tabular-nums text-muted-foreground">{{ n(r.value) }}</span>
        </div>
        <svg class="mt-1 block h-1.5 w-full" aria-hidden="true">
          <rect width="100%" height="6" rx="3" fill="var(--muted)" />
          <rect :width="`${pct(r.value)}%`" height="6" rx="3" fill="var(--primary)" />
        </svg>
      </component>
    </li>
  </ol>
</template>
