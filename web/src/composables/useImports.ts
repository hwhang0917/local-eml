import { computed, reactive, ref } from 'vue'
import { toast } from 'vue-sonner'
import { api, type ImportEvent, type S3ImportConfig, type IMAPImportConfig } from '@/lib/api'
import { i18n } from '@/i18n'

const t = (key: string, params?: Record<string, unknown>) =>
  i18n.global.t(key, params ?? {})

// Newest first; older entries fall off so a million-message import can't
// grow the page without bound.
const MAX_LOG_ENTRIES = 200

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

// The server names export jobs with a '-export' suffix on the kind; that is the
// only thing telling the two pages' job lists (and toast namespaces) apart.
const isExport = (r: ImportRun) => r.kind.endsWith('-export')

const importRuns = computed(() => runs.value.filter((r) => !isExport(r)))
const exportRuns = computed(() => runs.value.filter(isExport))

// Module scope like runs itself: leaving the page and coming back mid-job must
// still show the start buttons as busy.
const importActive = computed(() => importRuns.value.some((r) => !r.done))
const exportActive = computed(() => exportRuns.value.some((r) => !r.done))

async function hydrateFromDB(run: ImportRun) {
  try {
    const s = await api.importStatus(run.id)
    if (s.total) run.total = s.total
    run.processed = Math.max(run.processed, s.processed)
    run.duplicates = Math.max(run.duplicates, s.duplicates)
    run.errors = Math.max(run.errors, s.errors)
  } catch {
    // The live counters are already on screen; the DB row only tightens them.
  }
}

function notify(run: ImportRun) {
  const ns = isExport(run) ? 'export' : 'import'
  const desc = t(`${ns}.toast_summary`, {
    processed: run.processed,
    dup: run.duplicates,
    err: run.errors,
  })
  if (run.errors > 0) {
    toast.error(t(`${ns}.toast_error_title`), { description: desc })
  } else {
    toast.success(t(`${ns}.toast_done_title`), { description: desc })
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
      if (run.log.length > MAX_LOG_ENTRIES) run.log.length = MAX_LOG_ENTRIES
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
      run.log.unshift({
        path: t('import.log_job'),
        kind: 'err',
        detail: ev.message || t('import.failed_fallback'),
      })
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

async function cancelRun(id: string) {
  try {
    await api.cancelJob(id)
  } catch (e) {
    toast.error(t('import.cancel_error'), { description: String(e) })
  }
}

function track(id: string, kind: string, phaseKey: string, total = 0) {
  const run = reactive<ImportRun>({
    id,
    kind,
    phase: t(phaseKey),
    current: '',
    total,
    processed: 0,
    duplicates: 0,
    errors: 0,
    done: false,
    log: [],
  })
  runs.value.unshift(run)
  followProgress(run)
}

export function useImports() {
  async function startImport(files: File[]) {
    const { import_id, kind } = await api.upload(files)
    // Archives report their real total once the server has scanned them.
    track(import_id, kind, 'import.queued', kind === 'file' || kind === 'dir' ? files.length : 0)
  }

  async function startS3Import(cfg: S3ImportConfig) {
    const { import_id, kind } = await api.uploadS3(cfg)
    track(import_id, kind, 'import.queued')
  }

  async function startImapImport(cfg: IMAPImportConfig) {
    const { import_id, kind } = await api.uploadImap(cfg)
    track(import_id, kind, 'import.queued')
  }

  async function startS3Export(cfg: S3ImportConfig) {
    const { export_id, kind } = await api.exportS3(cfg)
    track(export_id, kind, 'export.queued')
  }

  return {
    importRuns, exportRuns, importActive, exportActive,
    startImport, startS3Import, startImapImport, startS3Export, cancelRun,
  }
}
