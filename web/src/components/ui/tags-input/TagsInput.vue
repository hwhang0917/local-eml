<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  TagsInputRoot,
  type TagsInputRootEmits,
  type TagsInputRootProps,
  useForwardPropsEmits,
} from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputRootProps & { class?: HTMLAttributes['class'] }>()
const emits = defineEmits<TagsInputRootEmits>()
const delegated = reactiveOmit(props, 'class')
const forwarded = useForwardPropsEmits(delegated, emits)
</script>

<template>
  <TagsInputRoot
    v-bind="forwarded"
    :class="cn(
      'flex flex-wrap items-center gap-1.5 rounded-full border border-hairline bg-background px-3 py-1.5 min-h-9',
      'focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background',
      props.class,
    )"
  >
    <slot />
  </TagsInputRoot>
</template>
