<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'vue'

defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <div
    :class="cn(
      'flex items-center justify-between gap-2 px-4 pt-4 pb-2 border-b border-hairline',
      $props.class,
    )"
  >
    <slot />
  </div>
</template>
