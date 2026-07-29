<script setup lang="ts">
import {
  DropdownMenuContent,
  DropdownMenuPortal,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import CategoryDot from '@/components/ui/CategoryDot.vue'

export type CategoryOption = { value: string; label: string; color?: string }

defineProps<{ modelValue: string; options: CategoryOption[] }>()
const emit = defineEmits<{ select: [string] }>()
</script>

<!--
  A radio dropdown that draws dots. It knows nothing about categories, so the
  filter and the two assignment sites can pass different triggers and different
  option lists without a mode flag.

  DropdownMenu rather than Select because one caller's trigger is a bare dot in a
  table cell, which the SelectTrigger chrome would fight. Portalled, so menu
  clicks land outside the clickable <tr> and cannot bubble into row navigation.
-->
<template>
  <DropdownMenuRoot>
    <!-- The slot owns the accessible name: triggers with visible text use it
         directly, icon-only triggers set their own aria-label. A label here
         would override the visible text and mismatch it for screen readers. -->
    <DropdownMenuTrigger as-child>
      <slot name="trigger" />
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        :side-offset="6"
        align="start"
        class="z-50 min-w-44 rounded-lg border border-hairline bg-background p-1 shadow-lg"
      >
        <DropdownMenuRadioGroup
          :model-value="modelValue"
          @update:model-value="(v) => emit('select', String(v))"
        >
          <DropdownMenuRadioItem
            v-for="o in options"
            :key="o.value"
            :value="o.value"
            class="flex cursor-default select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none
              data-[highlighted]:bg-accent data-[state=checked]:font-medium"
          >
            <CategoryDot :color="o.color" />
            <span class="truncate">{{ o.label }}</span>
          </DropdownMenuRadioItem>
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
