<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'vue'

defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <aside
    :class="cn(
      'w-60 shrink-0 self-start rounded-lg border border-hairline bg-card overflow-hidden',
      $props.class,
    )"
  >
    <slot />
  </aside>
</template>
