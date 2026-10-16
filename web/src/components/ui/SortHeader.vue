<script setup lang="ts" generic="Col extends string">
import { computed } from 'vue'

// Generic over the column key so the page's sort handler keeps its narrow type.
const props = withDefaults(
  defineProps<{
    label: string
    col: Col
    sort: Col
    order: string
    align?: 'left' | 'right'
  }>(),
  { align: 'left' },
)
const emit = defineEmits<{ sort: [col: Col] }>()

const active = computed(() => props.sort === props.col)
const ascending = computed(() => active.value && props.order === 'asc')
</script>

<!-- A bare <th onClick> is invisible to keyboards and screen readers. The
     standard sortable-header pattern is a real button inside the th, with
     aria-sort carrying the state the arrow shows visually. -->
<template>
  <th
    scope="col"
    :aria-sort="active ? (ascending ? 'ascending' : 'descending') : 'none'"
    :class="['px-3 py-2 select-none', align === 'right' ? 'text-right' : 'text-left']"
  >
    <button
      type="button"
      class="uppercase cursor-pointer hover:text-foreground rounded-xs
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      @click="emit('sort', col)"
    >
      {{ label }}<span aria-hidden="true">{{ active ? (ascending ? ' ↑' : ' ↓') : '' }}</span>
    </button>
  </th>
</template>
