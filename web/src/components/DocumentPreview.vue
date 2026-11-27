<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { api } from '@/lib/api'
import type { DocPreviewKind } from '@/lib/docPreview'
import type { HwpDocument } from '@rhwp/core'
import { SheetPreview, type SheetData } from '@/lib/sheetPreview'

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
  freeSheets()
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

// SheetJS runs in a worker (lib/sheetWorker.ts): a file built to stall the
// parser can't freeze the app, and is given up on after a timeout.
const SHEET_MAX_ROWS = 1000
const SHEET_MAX_COLS = 100
const sheetNames = ref<string[]>([])
const sheetName = ref('')
const sheetData = shallowRef<SheetData | null>(null)
let sheets: SheetPreview | null = null

function freeSheets() {
  sheets?.close()
  sheets = null
  sheetNames.value = []
  sheetName.value = ''
  sheetData.value = null
}

async function openSheet(bytes: Uint8Array, gen: number) {
  const preview = new SheetPreview(SHEET_MAX_ROWS, SHEET_MAX_COLS)
  sheets = preview
  const names = await preview.open(bytes)
  if (gen !== generation) return
  sheetNames.value = names
  await showSheet(names[0] ?? '', gen)
}

async function showSheet(name: string, gen = generation) {
  const preview = sheets
  if (!preview) return
  sheetName.value = name
  try {
    const data = await preview.sheet(name)
    if (gen === generation && sheets === preview && sheetName.value === name) sheetData.value = data
  } catch (e) {
    if (gen !== generation || sheets !== preview) return
    console.warn('document preview failed', e)
    status.value = 'error'
  }
}

const sheet = computed(() => {
  const d = sheetData.value
  if (!d) return null
  return { ...d, truncated: d.totalRows > SHEET_MAX_ROWS || d.totalCols > SHEET_MAX_COLS }
})

// Last, so the immediate first load sees every binding above initialized.
watch(() => [props.sha, props.index, props.kind], () => void load(), { immediate: true })
onBeforeUnmount(() => {
  generation++
  freeHwp()
  freeSheets()
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

    <div v-else-if="status === 'ready' && sheetData" class="flex flex-col">
      <div v-if="sheetNames.length > 1" class="flex gap-1 overflow-x-auto border-b p-1" role="tablist">
        <button
          v-for="name in sheetNames"
          :key="name"
          type="button"
          role="tab"
          :aria-selected="name === sheetName"
          class="shrink-0 rounded-sm px-3 py-1 text-sm"
          :class="name === sheetName ? 'bg-accent font-medium text-foreground' : 'text-muted-foreground hover:bg-accent/60'"
          @click="showSheet(name)"
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
