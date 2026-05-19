<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{ modelValue?: string; placeholder?: string; type?: string }>()
defineEmits<{ (e: 'update:modelValue', v: string): void }>()
</script>

<template>
  <input
    :type="type ?? 'text'"
    :value="modelValue"
    :placeholder="placeholder"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    :class="cn(
      'flex h-10 w-full rounded-full border border-hairline bg-background px-4 text-sm',
      'placeholder:text-muted-foreground',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
      'disabled:cursor-not-allowed disabled:opacity-50',
    )"
  />
</template>
