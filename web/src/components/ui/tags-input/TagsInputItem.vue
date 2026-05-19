<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  TagsInputItem,
  type TagsInputItemProps,
  useForwardProps,
} from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<TagsInputItemProps & { class?: HTMLAttributes['class'] }>()
const delegated = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegated)
</script>

<template>
  <TagsInputItem
    v-bind="forwarded"
    :class="cn(
      'inline-flex items-center gap-1.5 rounded-full bg-accent text-accent-foreground pl-2.5 pr-1.5 py-0.5 text-xs',
      'data-[state=active]:ring-2 data-[state=active]:ring-ring',
      props.class,
    )"
  >
    <slot />
  </TagsInputItem>
</template>
