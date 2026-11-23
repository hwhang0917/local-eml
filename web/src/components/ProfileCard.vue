<script setup lang="ts">
import { cn } from '@/lib/utils'

defineProps<{
  name: string
  /** Second line under the name, in monospace: where the profile points. */
  detail: string
  selected: boolean
  /** Tooltip on the clickable part of the card. */
  selectTitle: string
  /** Colour class for the dot before the name; leave out for no dot. */
  dotClass?: string
}>()
const emit = defineEmits<{ select: [] }>()
</script>

<!--
  The shell shared by the saved-profile cards (IMAP and S3): clicking the
  name loads the profile into the form below, the `action` slot sits at the
  top right (a switch, say) and the `footer` slot holds the status line.
-->
<template>
  <div
    :class="cn(
      'rounded-md border bg-card p-3 flex flex-col gap-2 transition-colors',
      selected ? 'border-primary ring-1 ring-primary' : 'border-border hover:border-muted-foreground/50',
    )"
  >
    <div class="flex items-start gap-3">
      <button
        type="button"
        class="flex-1 min-w-0 text-left cursor-pointer focus-visible:outline-none focus-visible:underline"
        :aria-pressed="selected"
        :title="selectTitle"
        @click="emit('select')"
      >
        <span class="flex items-center gap-2">
          <span v-if="dotClass" :class="cn('inline-block h-2 w-2 rounded-full shrink-0', dotClass)" aria-hidden="true" />
          <span class="font-medium truncate">{{ name }}</span>
        </span>
        <span class="block text-xs text-muted-foreground font-mono truncate mt-0.5" :title="detail">
          {{ detail }}
        </span>
      </button>
      <slot name="action" />
    </div>

    <div class="flex items-center gap-2 text-xs min-h-8">
      <slot name="footer" />
    </div>
  </div>
</template>
