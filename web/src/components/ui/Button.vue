<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<{
    variant?: 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive'
    size?: 'sm' | 'md' | 'lg' | 'icon'
    type?: 'button' | 'submit' | 'reset'
    disabled?: boolean
  }>(),
  { variant: 'default', size: 'md', type: 'button', disabled: false },
)

const cls = computed(() => {
  const base = 'inline-flex items-center justify-center gap-2 whitespace-nowrap font-normal'
    + ' focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
    + ' focus-visible:ring-offset-background disabled:opacity-50 disabled:pointer-events-none select-none'

  const variants = {
    default: 'bg-primary text-primary-foreground rounded-full hover:bg-primary/95',
    secondary: 'bg-transparent text-primary border border-primary rounded-full hover:bg-primary/5',
    outline: 'bg-pearl text-foreground border border-hairline hover:bg-accent rounded-sm',
    ghost: 'bg-transparent text-foreground hover:bg-accent rounded-sm',
    destructive: 'bg-destructive text-destructive-foreground rounded-full hover:bg-destructive/95',
  }

  const sizes = {
    sm: 'h-8 px-4 text-sm',
    md: 'h-10 px-5 text-sm',
    lg: 'h-11 px-6 text-base',
    icon: 'h-10 w-10 rounded-sm',
  }

  return cn(base, variants[props.variant], sizes[props.size])
})
</script>

<template>
  <button :type="type" :disabled="disabled" :class="cls">
    <slot />
  </button>
</template>
