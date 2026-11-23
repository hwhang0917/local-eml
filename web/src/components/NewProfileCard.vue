<script setup lang="ts">
import { Plus } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

defineProps<{ selected: boolean; label: string }>()
const emit = defineEmits<{ select: [] }>()
</script>

<!-- The dashed "New profile" tile that ends a grid of profile cards. -->
<template>
  <button
    type="button"
    :class="cn(
      'rounded-md border border-dashed p-3 min-h-[5.5rem] flex items-center justify-center gap-2 text-sm transition-colors cursor-pointer',
      selected
        ? 'border-primary text-foreground bg-accent/40'
        : 'border-border text-muted-foreground hover:text-foreground hover:border-muted-foreground/50',
    )"
    :aria-pressed="selected"
    @click="emit('select')"
  >
    <Plus class="h-4 w-4" aria-hidden="true" />
    {{ label }}
  </button>
</template>
