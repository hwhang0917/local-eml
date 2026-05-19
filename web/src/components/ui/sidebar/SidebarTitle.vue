<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'vue'

defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <h3
    :class="cn(
      'text-xs font-semibold uppercase tracking-wide text-muted-foreground',
      $props.class,
    )"
  >
    <slot />
  </h3>
</template>
