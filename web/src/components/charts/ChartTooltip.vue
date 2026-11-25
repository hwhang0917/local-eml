<script setup lang="ts">
import { computed, ref } from 'vue'
import { useElementSize } from '@vueuse/core'

// A small card that follows the pointer inside a chart. x and y are in the
// parent's pixel space (the parent must be position: relative); the card
// flips to the left of the pointer near the right edge.
const props = defineProps<{ x: number; y: number; containerWidth: number }>()

const el = ref<HTMLElement | null>(null)
const { width } = useElementSize(el)
const GAP = 12

const style = computed(() => {
  const flip = props.x + GAP + width.value > props.containerWidth
  const left = flip ? Math.max(0, props.x - GAP - width.value) : props.x + GAP
  return { left: `${left}px`, top: `${Math.max(0, props.y - 8)}px` }
})
</script>

<template>
  <div
    ref="el"
    role="tooltip"
    class="pointer-events-none absolute z-10 whitespace-nowrap rounded-md border border-hairline bg-card px-2.5 py-1.5 text-xs text-card-foreground shadow-md"
    :style="style"
  >
    <slot />
  </div>
</template>
