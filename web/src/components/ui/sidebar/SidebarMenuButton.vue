<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'vue'

withDefaults(
  defineProps<{
    active?: boolean
    class?: HTMLAttributes['class']
  }>(),
  { active: false },
)
</script>

<template>
  <button
    type="button"
    :data-active="active"
    :class="cn(
      'flex w-full items-center gap-2 rounded-sm px-3 py-1.5 text-sm text-foreground',
      'hover:bg-accent hover:text-accent-foreground',
      'data-[active=true]:bg-accent data-[active=true]:font-medium data-[active=true]:text-accent-foreground',
      $props.class,
    )"
  >
    <slot />
  </button>
</template>
