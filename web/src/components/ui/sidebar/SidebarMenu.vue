<script setup lang="ts">
import { cn } from '@/lib/utils'
import type { HTMLAttributes } from 'vue'

defineProps<{ class?: HTMLAttributes['class'] }>()
</script>

<template>
  <ul :class="cn('flex flex-col gap-0.5', $props.class)">
    <slot />
  </ul>
</template>
