import { reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import { api, type ImportEvent, type S3ImportConfig, type IMAPImportConfig } from '@/lib/api'
import { i18n } from '@/i18n'

const t = (key: string, params?: Record<string, unknown>) =>
  i18n.global.t(key, params ?? {})

export interface LogEntry {
  path: string
  kind: 'ok' | 'dup' | 'err'
  detail: string
}

export interface ImportRun {
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

function notify(run: ImportRun) {
  const desc = t('import.toast_summary', {
    processed: run.processed,
    dup: run.duplicates,
    err: run.errors,
  })
  if (run.errors > 0) {
    toast.error(t('import.toast_error_title'), { description: desc })
  } else {
    toast.success(t('import.toast_done_title'), { description: desc })
  }
}

function finish(run: ImportRun) {
  hydrateFromDB(run).then(() => notify(run))
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
      run.phase = ev.phase || t('import.completed_fallback')
      run.current = ''
      run.done = true
      es.close()
      finish(run)
    } else if (ev.type === 'error') {
      run.errors++
      run.phase = ev.phase || t('import.failed_fallback')
      run.current = ''
      run.done = true
      run.log.unshift({ path: '(job)', kind: 'err', detail: ev.message || 'error' })
      es.close()
      finish(run)
    }
  }
  es.onerror = () => {
    if (!run.done) {
      run.done = true
      finish(run)
    }
    es.close()
  }
}

export function useImports() {
  async function startImport(files: File[]) {
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

  async function startS3Import(cfg: S3ImportConfig) {
    const { import_id, kind } = await api.uploadS3(cfg)
    const run = reactive<ImportRun>({
      id: import_id,
      kind,
      phase: t('import.queued'),
      current: '',
      total: 0,
      processed: 0,
      duplicates: 0,
      errors: 0,
      done: false,
      log: [],
    })
    runs.value.unshift(run)
    followProgress(run)
  }

  async function startImapImport(cfg: IMAPImportConfig) {
    const { import_id, kind } = await api.uploadImap(cfg)
    const run = reactive<ImportRun>({
      id: import_id,
      kind,
      phase: t('import.queued'),
      current: '',
      total: 0,
      processed: 0,
      duplicates: 0,
      errors: 0,
      done: false,
      log: [],
    })
    runs.value.unshift(run)
    followProgress(run)
  }

  return { runs, startImport, startS3Import, startImapImport }
}
