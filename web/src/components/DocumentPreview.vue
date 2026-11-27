<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/lib/api'
import type { DocPreviewKind } from '@/lib/docPreview'
import type { HwpDocument } from '@rhwp/core'
import type { WorkBook } from 'xlsx'

// Renders HWP/HWPX, DOCX and spreadsheet attachments entirely in the browser.
// Nothing leaves the machine: the bytes come from our own API and each
// renderer is a lazily loaded chunk bundled into the app.
const props = defineProps<{ sha: string; index: number; kind: DocPreviewKind }>()
const { t, n } = useI18n()

const status = ref<'loading' | 'ready' | 'error'>('loading')
let generation = 0


async function load() {
  const gen = ++generation
  status.value = 'loading'
  freeHwp()
  try {
    const bytes = await api.getAttachmentBytes(props.sha, props.index)
    if (gen !== generation) return
    if (props.kind === 'hwp') await openHwp(bytes, gen)
    else if (props.kind === 'docx') await openDocx(bytes, gen)
    else await openSheet(bytes, gen)
    if (gen === generation) status.value = 'ready'
  } catch (e) {
    if (gen !== generation) return
    console.warn('document preview failed', e)
    status.value = 'error'
  }
}

// --- HWP / HWPX (rhwp, Rust compiled to WASM) ---

const HWP_PAGE_BATCH = 20
let rhwp: Promise<typeof import('@rhwp/core')> | null = null
function loadRhwp() {
  rhwp ??= (async () => {
    const m = await import('@rhwp/core')
    // rhwp lays text out with the browser's own font metrics.
    let ctx: CanvasRenderingContext2D | null = null
    let lastFont = ''
    ;(globalThis as { measureTextWidth?: (font: string, text: string) => number }).measureTextWidth = (font, text) => {
      ctx ??= document.createElement('canvas').getContext('2d')!
      if (font !== lastFont) {
        ctx.font = font
        lastFont = font
      }
      return ctx.measureText(text).width
    }
    await m.default()
    return m
  })().catch((e) => {
    rhwp = null
    throw e
  })
  return rhwp
}

const hwpPages = ref<HTMLElement | null>(null)
const hwpTotal = ref(0)
const hwpShown = ref(0)
const hwpRendering = ref(false)
let hwpDoc: HwpDocument | null = null

function freeHwp() {
  hwpDoc?.free()
  hwpDoc = null
  hwpTotal.value = 0
  hwpShown.value = 0
  if (hwpPages.value) hwpPages.value.replaceChildren()
}

async function openHwp(bytes: Uint8Array, gen: number) {
  const m = await loadRhwp()
  if (gen !== generation) return
  hwpDoc = new m.HwpDocument(bytes)
  hwpTotal.value = hwpDoc.pageCount()
  status.value = 'ready'
  await nextTick()
  await renderMoreHwp(gen)
}

async function renderMoreHwp(gen = generation) {
  const doc = hwpDoc
  const host = hwpPages.value
  if (!doc || !host || hwpRendering.value) return
  hwpRendering.value = true
  try {
    const scale = Math.min(window.devicePixelRatio || 1, 2)
    const end = Math.min(hwpShown.value + HWP_PAGE_BATCH, hwpTotal.value)
    for (let i = hwpShown.value; i < end; i++) {
      const canvas = document.createElement('canvas')
      doc.renderPageToCanvas(i, canvas, scale)
      canvas.style.width = `${canvas.width / scale}px`
      canvas.className = 'mx-auto block h-auto max-w-full bg-white shadow-sm'
      canvas.setAttribute('role', 'img')
      canvas.setAttribute('aria-label', t('viewer.doc_preview.page', { page: i + 1, total: hwpTotal.value }))
      host.append(canvas)
      hwpShown.value = i + 1
      // Yield between pages so a long document doesn't freeze the tab.
      await new Promise(requestAnimationFrame)
      if (gen !== generation || hwpDoc !== doc) return
    }
  } finally {
    hwpRendering.value = false
  }
}

// --- DOCX (docx-preview) ---

// The document is rendered into a sandboxed, script-less frame: its styles
// can't leak into the app, and the CSP stops it from fetching anything.
const DOCX_FRAME = `<!doctype html><html><head><meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:">
<style>html,body{margin:0;background:#e5e5e5}.docx-wrapper{padding:16px!important}</style>
</head><body></body></html>`
const docxFrame = ref<HTMLIFrameElement | null>(null)

async function frameDocument(frame: HTMLIFrameElement): Promise<Document> {
  const ready = () => frame.contentDocument?.readyState === 'complete' && frame.contentDocument.body
  if (!ready()) await new Promise((r) => frame.addEventListener('load', r, { once: true }))
  return frame.contentDocument!
}

async function openDocx(bytes: Uint8Array, gen: number) {
  const { renderAsync } = await import('docx-preview')
  if (gen !== generation) return
  status.value = 'ready'
  await nextTick()
  const frame = docxFrame.value
  if (!frame) return
  const doc = await frameDocument(frame)
  doc.body.replaceChildren()
  doc.head.querySelectorAll('style:not(:first-of-type)').forEach((s) => s.remove())
  await renderAsync(bytes, doc.body, doc.head, {
    inWrapper: true,
    useBase64URL: true,
    renderAltChunks: false,
    renderChanges: false,
    experimental: false,
  })
  // Links stay inert: a click would otherwise load a remote page in the frame.
  doc.addEventListener('click', (e) => {
    const a = (e.target as Element | null)?.closest?.('a')
    if (a && !a.getAttribute('href')?.startsWith('#')) e.preventDefault()
  })
}

// --- Spreadsheets: XLSX, XLSM, XLS, ODS (SheetJS) ---

const SHEET_MAX_ROWS = 1000
const SHEET_MAX_COLS = 100
const workbook = shallowRef<WorkBook | null>(null)
const sheetName = ref('')
let xlsx: typeof import('xlsx') | null = null

async function openSheet(bytes: Uint8Array, gen: number) {
  if (!xlsx) {
    const [m, cp] = await Promise.all([import('xlsx'), import('xlsx/dist/cpexcel.full.mjs')])
    // Legacy .xls files in Korean (CP949) and other code pages need the tables.
    m.set_cptable(cp)
    xlsx = m
  }
  if (gen !== generation) return
  const wb = xlsx.read(bytes, { type: 'array', dense: true, sheetRows: SHEET_MAX_ROWS + 1, cellHTML: false })
  workbook.value = wb
  sheetName.value = wb.SheetNames[0] ?? ''
}

interface SheetCell { text: string; numeric: boolean }

const sheet = computed(() => {
  const wb = workbook.value
  const ws = wb && xlsx ? wb.Sheets[sheetName.value] : undefined
  if (!ws || !xlsx) return null
  const full = xlsx.utils.decode_range(ws['!fullref'] ?? ws['!ref'] ?? 'A1')
  const used = xlsx.utils.decode_range(ws['!ref'] ?? 'A1')
  const totalRows = full.e.r + 1
  const totalCols = full.e.c + 1
  const rowCount = Math.min(used.e.r + 1, SHEET_MAX_ROWS)
  const cols = Math.min(totalCols, SHEET_MAX_COLS)
  // Read with dense: true, so cells are rows of arrays. format_cell applies
  // the sheet's number format (thousands separators, dates, percentages).
  const data = (ws['!data'] ?? []) as (import('xlsx').CellObject | undefined)[][]
  const rows: SheetCell[][] = []
  for (let r = 0; r < rowCount; r++) {
    const src = data[r] ?? []
    const row: SheetCell[] = []
    for (let c = 0; c < cols; c++) {
      const cell = src[c]
      row.push(cell ? { text: xlsx.utils.format_cell(cell), numeric: cell.t === 'n' } : { text: '', numeric: false })
    }
    rows.push(row)
  }
  return {
    rows,
    headers: Array.from({ length: cols }, (_, i) => xlsx!.utils.encode_col(i)),
    totalRows,
    totalCols,
    truncated: totalRows > SHEET_MAX_ROWS || totalCols > SHEET_MAX_COLS,
  }
})

// Last, so the immediate first load sees every binding above initialized.
watch(() => [props.sha, props.index, props.kind], () => void load(), { immediate: true })
onBeforeUnmount(() => {
  generation++
  freeHwp()
})
</script>

<template>
  <div class="rounded-md border">
    <p v-if="status === 'loading'" class="p-4 text-sm text-muted-foreground" role="status">
      {{ t('viewer.doc_preview.loading') }}
    </p>
    <p v-else-if="status === 'error'" class="p-4 text-sm text-muted-foreground" role="alert">
      {{ t('viewer.doc_preview.failed') }}
    </p>

    <template v-if="kind === 'hwp'">
      <div v-show="status === 'ready'" class="max-h-[70vh] overflow-auto bg-muted/40 p-4">
        <div ref="hwpPages" class="space-y-4"></div>
        <div v-if="hwpShown < hwpTotal" class="mt-4 flex items-center justify-center gap-3 text-sm text-muted-foreground">
          <span>{{ t('viewer.doc_preview.pages_shown', { shown: n(hwpShown), total: n(hwpTotal) }) }}</span>
          <button type="button" class="underline disabled:opacity-50" :disabled="hwpRendering" @click="renderMoreHwp()">
            {{ t('viewer.doc_preview.more_pages') }}
          </button>
        </div>
      </div>
    </template>

    <iframe
      v-else-if="kind === 'docx'"
      v-show="status === 'ready'"
      ref="docxFrame"
      sandbox="allow-same-origin"
      :srcdoc="DOCX_FRAME"
      :title="t('viewer.doc_preview.frame_title')"
      class="block h-[70vh] w-full rounded-md"
    ></iframe>

    <div v-else-if="status === 'ready' && workbook" class="flex flex-col">
      <div v-if="workbook.SheetNames.length > 1" class="flex gap-1 overflow-x-auto border-b p-1" role="tablist">
        <button
          v-for="name in workbook.SheetNames"
          :key="name"
          type="button"
          role="tab"
          :aria-selected="name === sheetName"
          class="shrink-0 rounded-sm px-3 py-1 text-sm"
          :class="name === sheetName ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:bg-accent/60'"
          @click="sheetName = name"
        >{{ name }}</button>
      </div>
      <div class="max-h-[70vh] overflow-auto">
        <table v-if="sheet && sheet.rows.length" class="border-collapse text-xs tabular-nums">
          <thead class="sticky top-0 z-10 bg-muted">
            <tr>
              <th class="sticky left-0 z-10 border bg-muted px-2 py-1"></th>
              <th v-for="h in sheet.headers" :key="h" class="border px-2 py-1 font-medium text-muted-foreground">{{ h }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(row, r) in sheet.rows" :key="r">
              <th class="sticky left-0 border bg-muted px-2 py-1 text-right font-medium text-muted-foreground">{{ r + 1 }}</th>
              <td v-for="(cell, c) in row" :key="c" class="max-w-80 truncate border px-2 py-1 whitespace-nowrap"
                :class="cell.numeric && 'text-right'" :title="cell.text || undefined">{{ cell.text }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="p-4 text-sm text-muted-foreground">{{ t('viewer.doc_preview.empty_sheet') }}</p>
      </div>
      <p v-if="sheet?.truncated" class="border-t px-3 py-2 text-xs text-muted-foreground">
        {{ t('viewer.doc_preview.sheet_truncated', { rows: n(Math.min(sheet.totalRows, SHEET_MAX_ROWS)), cols: n(Math.min(sheet.totalCols, SHEET_MAX_COLS)), totalRows: n(sheet.totalRows), totalCols: n(sheet.totalCols) }) }}
      </p>
    </div>
  </div>
</template>
