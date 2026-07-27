<script setup lang="ts">
import { computed } from 'vue'
import { dotClass } from '@/lib/categories'

const props = defineProps<{ color?: string; name?: string; size?: 'sm' | 'md' }>()

// No colour means "uncategorized": a hollow ring rather than a filled dot, so an
// unassigned row reads as empty instead of as a grey category.
const cls = computed(() => [
  'inline-block shrink-0 rounded-full',
  props.size === 'md' ? 'h-3 w-3' : 'h-2.5 w-2.5',
  props.color ? dotClass(props.color) : 'border border-dashed border-muted-foreground/60',
])
</script>

<template>
  <span :class="cls" role="img" :aria-label="name" :title="name" />
</template>
