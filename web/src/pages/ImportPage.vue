<script setup lang="ts">
import { ref, computed } from 'vue'
import { api, type ImportEvent } from '@/lib/api'
import { cn } from '@/lib/utils'
import Card from '@/components/ui/Card.vue'
import Button from '@/components/ui/Button.vue'

interface ImportRun {
  id: string
  kind: string
  total: number
  processed: number
  duplicates: number
  errors: number
  done: boolean
  log: Array<{ path: string; kind: 'ok' | 'dup' | 'err'; detail: string }>
}

const runs = ref<ImportRun[]>([])
const dragOver = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)
const dirInput = ref<HTMLInputElement | null>(null)

async function startUpload(files: File[]) {
  if (files.length === 0) return
  const { import_id, kind } = await api.upload(files)
  const run: ImportRun = {
    id: import_id,
    kind,
    // For zip uploads the real total isn't known until the server has walked
    // the archive — start at 0 so the bar doesn't jump from "0/1" to "42/42".
    total: kind === 'zip' ? 0 : files.length,
    processed: 0,
    duplicates: 0,
    errors: 0,
    done: false,
    log: [],
  }
  runs.value.unshift(run)
  followProgress(run)
}

function followProgress(run: ImportRun) {
  const es = api.importEventSource(run.id)
  es.onmessage = (msg) => {
    let ev: ImportEvent
    try { ev = JSON.parse(msg.data) } catch { return }
    if (ev.type === 'start' || ev.type === 'item') {
      if (ev.total) run.total = ev.total
      run.processed = ev.processed
      if (ev.type === 'item') {
        if (ev.duplicate) run.duplicates++
        if (ev.message) run.errors++
        run.log.unshift({
          path: ev.path || '',
          kind: ev.message ? 'err' : ev.duplicate ? 'dup' : 'ok',
          detail: ev.message || ev.sha256 || '',
        })
        if (run.log.length > 200) run.log.length = 200
      }
    } else if (ev.type === 'done') {
      if (ev.total) run.total = ev.total
      if (ev.processed) run.processed = ev.processed
      run.done = true
      es.close()
      // Fast jobs may finish before the SSE connects, so the only event we
      // get is a synthetic 'done' that lacks per-item history. Pull the final
      // counters from the DB to surface dup/error totals.
      hydrateFromDB(run)
    } else if (ev.type === 'error') {
      run.errors++
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
  } catch { /* ignore */ }
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
      <p class="text-lg mb-2">Drop <code>.eml</code> files, a directory, or a <code>.zip</code> here</p>
      <p class="text-sm text-muted-foreground mb-4">Duplicates are detected by SHA-256 and skipped.</p>
      <div class="flex justify-center gap-2">
        <Button variant="outline" @click="fileInput?.click()">Choose files…</Button>
        <Button variant="outline" @click="dirInput?.click()">Choose folder…</Button>
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
          <span class="font-medium">Import {{ run.id.slice(0, 8) }}</span>
          <span class="text-xs px-1.5 py-0.5 rounded bg-accent">{{ run.kind }}</span>
          <span class="ml-auto text-sm text-muted-foreground">
            {{ run.processed }} / {{ run.total }}
            <span v-if="run.duplicates"> · {{ run.duplicates }} dup</span>
            <span v-if="run.errors" class="text-destructive"> · {{ run.errors }} err</span>
            <span v-if="run.done" class="ml-2">✓ done</span>
          </span>
        </div>
        <div class="h-1.5 bg-muted rounded">
          <div class="h-full bg-primary rounded transition-all"
            :style="{ width: `${run.total ? (run.processed / run.total) * 100 : 0}%` }"></div>
        </div>
        <details class="mt-3" v-if="run.log.length">
          <summary class="text-xs text-muted-foreground cursor-pointer">Log ({{ run.log.length }})</summary>
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
