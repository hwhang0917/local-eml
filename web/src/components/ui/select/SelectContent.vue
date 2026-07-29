<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  SelectContent,
  type SelectContentEmits,
  type SelectContentProps,
  SelectPortal,
  SelectViewport,
  useForwardPropsEmits,
} from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = withDefaults(
  defineProps<SelectContentProps & { class?: HTMLAttributes['class'] }>(),
  { position: 'popper', sideOffset: 6 },
)
const emits = defineEmits<SelectContentEmits>()
const delegated = reactiveOmit(props, 'class')
const forwarded = useForwardPropsEmits(delegated, emits)
</script>

<template>
  <SelectPortal>
    <SelectContent
      v-bind="forwarded"
      :class="cn(
        // No entrance animation: the zoom/slide combo read as a wiggle next to
        // the app's other menus, which all open statically.
        'relative z-50 max-h-96 min-w-[8rem] overflow-hidden rounded-md border border-hairline bg-card text-card-foreground shadow-md',
        position === 'popper' && 'w-(--reka-select-trigger-width)',
        props.class,
      )"
    >
      <SelectViewport
        :class="cn(
          'p-1',
          position === 'popper' && 'h-(--reka-select-trigger-height)',
        )"
      >
        <slot />
      </SelectViewport>
    </SelectContent>
  </SelectPortal>
</template>
