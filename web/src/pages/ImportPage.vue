<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { cn } from '@/lib/utils'
import { formatBytes } from '@/lib/format'
import { useImports } from '@/composables/useImports'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()
const { runs, startImport } = useImports()

const stagedFiles = ref<File[]>([])
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dirInput = ref<HTMLInputElement | null>(null)

const stagedTotal = computed(() =>
  stagedFiles.value.reduce((s, f) => s + f.size, 0),
)

const stagedSummary = computed(() => {
  const total = formatBytes(stagedTotal.value)
  return stagedFiles.value.length === 1
    ? t('import.confirm_summary_one', { total })
    : t('import.confirm_summary_many', { count: stagedFiles.value.length, total })
})

function stageFiles(files: File[]) {
  if (files.length === 0) return
  stagedFiles.value = files
}

function clearStaged() {
  stagedFiles.value = []
}

async function confirmUpload() {
  if (stagedFiles.value.length === 0) return
  const files = stagedFiles.value
  stagedFiles.value = []
  await startImport(files)
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (!files) return
  stageFiles(Array.from(files))
}

function onFilePicked(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  stageFiles(Array.from(input.files))
  input.value = ''
}

const dropzoneClass = computed(() =>
  cn(
    'border-2 border-dashed text-center p-12 transition-colors',
    dragOver.value ? 'border-primary bg-accent/50' : 'border-border',
  ),
)
</script>

<template>
  <div class="space-y-6">
    <Card
      v-if="stagedFiles.length === 0"
      :class="dropzoneClass"
      @dragover.prevent="dragOver = true"
      @dragenter.prevent="dragOver = true"
      @dragleave="dragOver = false"
      @drop="onDrop"
    >
      <p class="text-lg mb-2">{{ t('import.drop') }}</p>
      <p class="text-sm text-muted-foreground mb-4">{{ t('import.dedup_note') }}</p>
      <div class="flex justify-center gap-2">
        <Button variant="outline" @click="fileInput?.click()">{{ t('import.choose_files') }}</Button>
        <Button variant="outline" @click="dirInput?.click()">{{ t('import.choose_folder') }}</Button>
        <input ref="fileInput" type="file" multiple accept=".eml,.zip" class="hidden" @change="onFilePicked" />
        <input
          ref="dirInput"
          type="file"
          multiple
          class="hidden"
          @change="onFilePicked"
          v-bind="{ webkitdirectory: true, directory: true } as any"
        />
      </div>
    </Card>

    <Card v-else class="p-6">
      <div class="flex items-start justify-between gap-4 mb-4">
        <div>
          <h3 class="text-lg font-semibold mb-1">{{ t('import.confirm_title') }}</h3>
          <p class="text-sm text-muted-foreground">{{ stagedSummary }}</p>
        </div>
        <div class="flex gap-2 shrink-0">
          <Button variant="outline" @click="clearStaged">{{ t('import.cancel') }}</Button>
          <Button @click="confirmUpload">{{ t('import.confirm') }}</Button>
        </div>
      </div>
      <ul class="text-xs font-mono space-y-1 max-h-56 overflow-auto border border-hairline rounded-sm p-3 bg-muted/30">
        <li v-for="(f, i) in stagedFiles" :key="i" class="flex justify-between gap-3">
          <span class="truncate" :title="f.name">{{ f.name }}</span>
          <span class="text-muted-foreground shrink-0">{{ formatBytes(f.size) }}</span>
        </li>
      </ul>
    </Card>

    <div v-for="run in runs" :key="run.id" class="space-y-2">
      <Card class="p-4">
        <div class="flex items-center gap-3 mb-2">
          <span class="font-medium">{{ t('import.import_label', { id: run.id.slice(0, 8) }) }}</span>
          <span class="text-xs px-1.5 py-0.5 rounded bg-accent">{{ run.kind }}</span>
          <span class="ml-auto text-sm text-muted-foreground">
            <span v-if="run.total">{{ run.processed }} / {{ run.total }}</span>
            <span v-if="run.duplicates"> · {{ run.duplicates }} {{ t('import.dup') }}</span>
            <span v-if="run.errors" class="text-destructive"> · {{ run.errors }} {{ t('import.err') }}</span>
          </span>
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

        <div class="h-1.5 bg-muted rounded overflow-hidden">
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
              <span class="inline-block w-12">{{ e.kind }}</span>
              <span>{{ e.path }}</span>
              <span v-if="e.detail" class="ml-2 text-muted-foreground">{{ e.detail.slice(0, 80) }}</span>
            </li>
          </ul>
        </details>
      </Card>
    </div>
  </div>
</template>
