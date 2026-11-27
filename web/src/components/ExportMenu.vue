<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Download, FileArchive, Mailbox, Printer } from 'lucide-vue-next'
import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuPortal,
  DropdownMenuRoot,
  DropdownMenuTrigger,
} from 'reka-ui'
import type { ExportFormat } from '@/composables/useSelectionExport'

// The three ways to take messages out of the library: as files, as one mbox
// another mail app can open, or as a printout the browser can save as PDF.

defineProps<{ label: string; triggerClass: string; disabled?: boolean; testid?: string }>()
const emit = defineEmits<{ pick: [format: ExportFormat] }>()
const { t } = useI18n()

const ITEMS: { format: ExportFormat; icon: typeof Download }[] = [
  { format: 'zip', icon: FileArchive },
  { format: 'mbox', icon: Mailbox },
  { format: 'pdf', icon: Printer },
]
</script>

<template>
  <DropdownMenuRoot>
    <DropdownMenuTrigger :class="triggerClass" :disabled="disabled" :data-testid="testid">
      <Download class="h-3.5 w-3.5" />
      {{ label }}
    </DropdownMenuTrigger>
    <DropdownMenuPortal>
      <DropdownMenuContent
        :side-offset="6"
        align="end"
        class="z-50 w-72 rounded-lg border border-hairline bg-background p-1 shadow-lg normal-case"
      >
        <DropdownMenuItem
          v-for="it in ITEMS"
          :key="it.format"
          :data-testid="`export-${it.format}`"
          class="flex cursor-default select-none items-start gap-2.5 rounded-sm px-2 py-1.5 text-sm outline-none
            data-[highlighted]:bg-accent"
          @select="emit('pick', it.format)"
        >
          <component :is="it.icon" class="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
          <span class="grid">
            <span class="font-medium text-foreground">{{ t(`export_selection.format.${it.format}`) }}</span>
            <span class="text-xs text-muted-foreground">{{ t(`export_selection.format.${it.format}_hint`) }}</span>
          </span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenuPortal>
  </DropdownMenuRoot>
</template>
