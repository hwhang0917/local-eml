<script setup lang="ts">
import { ref, computed, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { api, type ImportEvent } from '@/lib/api'
import { cn } from '@/lib/utils'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

const { t } = useI18n()

interface LogEntry {
  path: string
  kind: 'ok' | 'dup' | 'err'
  detail: string
}

interface ImportRun {
  id: string
  kind: string
  phase: string
  current: string
  total: number
  processed: number
  duplicates: number
  errors: number
  done: boolean
  log: LogEntry[]
}

const runs = ref<ImportRun[]>([])
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dirInput = ref<HTMLInputElement | null>(null)

async function startUpload(files: File[]) {
  if (files.length === 0) return
  const { import_id, kind } = await api.upload(files)
  const run = reactive<ImportRun>({
    id: import_id,
    kind,
    phase: t('import.queued'),
    current: '',
    total: kind === 'zip' ? 0 : files.length,
    processed: 0,
    duplicates: 0,
    errors: 0,
    done: false,
    log: [],
  })
  runs.value.unshift(run)
  followProgress(run)
}

function followProgress(run: ImportRun) {
  const es = api.importEventSource(run.id)
  es.onmessage = (msg) => {
    let ev: ImportEvent
    try { ev = JSON.parse(msg.data) } catch { return }
    if (ev.type === 'step') {
      if (ev.phase) run.phase = ev.phase
      if (ev.total) run.total = ev.total
    } else if (ev.type === 'item') {
      if (ev.total) run.total = ev.total
      run.processed = ev.processed
      run.current = ev.path?.split('/').pop() || ev.path || ''
      if (ev.duplicate) run.duplicates++
      if (ev.message) run.errors++
      run.log.unshift({
        path: ev.path || '',
        kind: ev.message ? 'err' : ev.duplicate ? 'dup' : 'ok',
        detail: ev.message || ev.sha256 || '',
      })
      if (run.log.length > 200) run.log.length = 200
    } else if (ev.type === 'done') {
      if (ev.total) run.total = ev.total
      if (ev.processed) run.processed = ev.processed
      run.phase = ev.phase || 'Completed'
      run.current = ''
      run.done = true
      es.close()
      hydrateFromDB(run)
    } else if (ev.type === 'error') {
      run.errors++
      run.phase = ev.phase || 'Failed'
      run.current = ''
      run.done = true
      run.log.unshift({ path: '(job)', kind: 'err', detail: ev.message || 'error' })
      es.close()
    }
  }
  es.onerror = () => {
    if (!run.done) hydrateFromDB(run)
    run.done = true
    es.close()
  }
}

async function hydrateFromDB(run: ImportRun) {
  try {
    const s = await api.importStatus(run.id)
    if (s.total) run.total = s.total
    run.processed = Math.max(run.processed, s.processed)
    run.duplicates = Math.max(run.duplicates, s.duplicates)
    run.errors = Math.max(run.errors, s.errors)
  } catch {
    /* */
  }
}

function onDrop(e: DragEvent) {
  e.preventDefault()
  dragOver.value = false
  const files = e.dataTransfer?.files
  if (!files) return
  startUpload(Array.from(files))
}

function onFilePicked(e: Event) {
  const input = e.target as HTMLInputElement
  if (!input.files) return
  startUpload(Array.from(input.files))
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
