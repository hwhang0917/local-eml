<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useImports, type ImportRun } from '@/composables/useImports'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

// Error messages can be whole stack traces; the log row shows a glimpse.
const DETAIL_MAX_CHARS = 80

const props = defineProps<{
  run: ImportRun
  /** `import.import_label` or `export.export_label`; takes `{ id }`. */
  labelKey: string
}>()

const { t, te } = useI18n()
const { cancelRun } = useImports()

const label = computed(() => t(props.labelKey, { id: props.run.id.slice(0, 8) }))
// Kinds are server enums; an unknown one is shown as sent rather than hidden.
const kindLabel = computed(() =>
  te(`import.kind.${props.run.kind}`) ? t(`import.kind.${props.run.kind}`) : props.run.kind,
)
</script>

<template>
  <Card class="p-4">
    <div class="flex items-center gap-3 mb-2">
      <span class="font-medium">{{ label }}</span>
      <span class="text-xs px-1.5 py-0.5 rounded bg-accent">{{ kindLabel }}</span>
      <span class="ml-auto text-sm text-muted-foreground">
        <span v-if="run.total">{{ run.processed }} / {{ run.total }}</span>
        <span v-if="run.duplicates"> · {{ run.duplicates }} {{ t('import.dup') }}</span>
        <span v-if="run.errors" class="text-destructive"> · {{ run.errors }} {{ t('import.err') }}</span>
      </span>
      <Button v-if="!run.done" size="sm" variant="outline" @click="cancelRun(run.id)">
        {{ t('import.abort') }}
      </Button>
    </div>

    <div class="flex items-center justify-between text-xs text-muted-foreground mb-1">
      <span :class="{ 'text-foreground': run.done && run.errors === 0 }">
        <span v-if="run.done && run.errors === 0">✓</span>
        <span v-else-if="run.done && run.errors > 0" class="text-destructive">✗</span>
        {{ run.phase }}
      </span>
      <span v-if="run.current" class="truncate ml-2 max-w-[50%]" :title="run.current">
        {{ run.current }}
      </span>
    </div>

    <div
      class="h-1.5 bg-muted rounded overflow-hidden"
      role="progressbar"
      :aria-label="label"
      :aria-valuemin="0"
      :aria-valuemax="run.total || undefined"
      :aria-valuenow="run.total ? run.processed : undefined"
    >
      <div
        class="h-full bg-primary transition-all"
        :class="{ 'animate-pulse': !run.done && run.total === 0 }"
        :style="{
          width: run.total
            ? `${Math.min(100, (run.processed / run.total) * 100)}%`
            : (run.done ? '100%' : '15%'),
        }"
      ></div>
    </div>

    <details class="mt-3" v-if="run.log.length">
      <summary class="text-xs text-muted-foreground cursor-pointer">{{ t('import.log') }} ({{ run.log.length }})</summary>
      <ul class="mt-2 text-xs font-mono space-y-0.5 max-h-48 overflow-auto">
        <li v-for="(e, i) in run.log" :key="i" :class="{
          'text-destructive': e.kind === 'err',
          'text-muted-foreground': e.kind === 'dup',
        }">
          <span class="inline-block w-12">{{ t(`import.log_kind.${e.kind}`) }}</span>
          <span>{{ e.path }}</span>
          <span v-if="e.detail" class="ml-2 text-muted-foreground">{{ e.detail.slice(0, DETAIL_MAX_CHARS) }}</span>
        </li>
      </ul>
    </details>
  </Card>
</template>
