<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { SlidersHorizontal } from 'lucide-vue-next'
import { PopoverArrow, PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'

// The search operators, as a cheat sheet beside the search box. Picking a row
// adds its operator to the query so nobody has to remember the spelling;
// rows with nothing sensible to insert ("exact phrase", "-word") just explain.

const emit = defineEmits<{ insert: [text: string] }>()
const { t } = useI18n()
const open = ref(false)

type Row = { example: string; key: string; insert?: string }
const ROWS: Row[] = [
  { example: 'from:alice', key: 'from', insert: 'from:' },
  { example: 'to:me@example.com', key: 'to', insert: 'to:' },
  { example: 'cc:bob', key: 'cc', insert: 'cc:' },
  { example: 'subject:invoice', key: 'subject', insert: 'subject:' },
  { example: 'filename:pdf', key: 'filename', insert: 'filename:' },
  { example: 'has:attachment', key: 'has_attachment', insert: 'has:attachment' },
  { example: 'is:starred', key: 'is_starred', insert: 'is:starred' },
  { example: 'is:unread', key: 'is_unread', insert: 'is:unread' },
  { example: 'after:2019-03', key: 'after', insert: 'after:' },
  { example: 'before:2020', key: 'before', insert: 'before:' },
  { example: 'on:2024-05-17', key: 'on', insert: 'on:' },
  { example: 'larger:5M', key: 'larger', insert: 'larger:' },
  { example: 'smaller:100K', key: 'smaller', insert: 'smaller:' },
  { example: '"exact phrase"', key: 'phrase' },
  { example: '-newsletter', key: 'exclude' },
]

function pick(r: Row) {
  if (!r.insert) return
  emit('insert', r.insert)
  open.value = false
}
</script>

<template>
  <PopoverRoot v-model:open="open">
    <PopoverTrigger
      :aria-label="t('search_help.open')"
      :title="t('search_help.open')"
      data-testid="search-help"
      class="inline-flex h-6 w-6 items-center justify-center rounded-sm text-muted-foreground hover:text-foreground
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <SlidersHorizontal class="h-4 w-4" />
    </PopoverTrigger>
    <PopoverPortal>
      <PopoverContent
        :side-offset="8"
        align="end"
        class="z-50 max-h-[var(--reka-popover-content-available-height)] w-[min(26rem,calc(100vw-2rem))] overflow-y-auto
          rounded-lg border border-hairline bg-background p-3 text-sm shadow-lg"
        @open-auto-focus.prevent
      >
        <PopoverArrow class="fill-background" />
        <p class="mb-1 font-semibold">{{ t('search_help.title') }}</p>
        <p class="mb-2 text-xs text-muted-foreground">{{ t('search_help.intro') }}</p>
        <ul class="grid">
          <li v-for="r in ROWS" :key="r.key">
            <component
              :is="r.insert ? 'button' : 'div'"
              :type="r.insert ? 'button' : undefined"
              :class="[
                'flex w-full items-baseline gap-3 rounded-sm px-2 py-0.5 text-left',
                r.insert ? 'cursor-pointer hover:bg-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring' : '',
              ]"
              @click="pick(r)"
            >
              <code class="w-32 shrink-0 font-mono text-xs text-foreground">{{ r.example }}</code>
              <span class="text-xs text-muted-foreground">{{ t(`search_help.rows.${r.key}`) }}</span>
            </component>
          </li>
        </ul>
        <p class="mt-2 border-t border-hairline pt-2 text-xs text-muted-foreground">{{ t('search_help.dates') }}</p>
      </PopoverContent>
    </PopoverPortal>
  </PopoverRoot>
</template>
