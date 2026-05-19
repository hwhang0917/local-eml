<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'outline' | 'ghost' | 'destructive'
    size?: 'sm' | 'md' | 'lg' | 'icon'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }>(),
  { variant: 'default', size: 'md', type: 'button', disabled: false },
)

const cls = computed(() =>
  cn(
    'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-md font-medium transition-colors',
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
    'disabled:opacity-50 disabled:pointer-events-none',
    {
      default: 'bg-primary text-primary-foreground hover:bg-primary/90',
      outline: 'border border-border bg-background hover:bg-accent hover:text-accent-foreground',
      ghost: 'hover:bg-accent hover:text-accent-foreground',
      destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
    }[props.variant],
    {
      sm: 'h-8 px-3 text-sm',
      md: 'h-9 px-4 text-sm',
      lg: 'h-10 px-6',
      icon: 'h-9 w-9',
    }[props.size],
  ),
)
</script>

<template>
  <button :type="type" :disabled="disabled" :class="cls">
    <slot />
  </button>
</template>
