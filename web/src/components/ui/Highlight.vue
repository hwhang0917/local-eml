<script setup lang="ts">
import { computed } from 'vue'
import { highlightSegments } from '@/lib/highlight'

const props = defineProps<{ text: string; query?: string }>()
const segments = computed(() => highlightSegments(props.text, props.query ?? ''))
</script>

<!-- <mark> is the element the platform already has for "matched here", so
     find-in-page styling and assistive tech get it for free. -->
<template>
  <template v-for="(seg, i) in segments" :key="i">
    <mark v-if="seg.match" class="rounded-xs bg-amber-200 text-inherit dark:bg-amber-400/30">{{ seg.text }}</mark>
    <template v-else>{{ seg.text }}</template>
  </template>
</template>
