<script setup lang="ts">
import { reactiveOmit } from '@vueuse/core'
import {
  TagsInputInput,
  type TagsInputInputProps,
  useForwardProps,
} from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'

const props = defineProps<
  TagsInputInputProps & { class?: HTMLAttributes['class']; placeholder?: string }
>()
const delegated = reactiveOmit(props, 'class')
const forwarded = useForwardProps(delegated)
</script>

<template>
  <TagsInputInput
    v-bind="forwarded"
    :placeholder="placeholder"
    :class="cn(
      'flex-1 min-w-[8ch] outline-none bg-transparent text-sm placeholder:text-muted-foreground',
      props.class,
    )"
  />
</template>
