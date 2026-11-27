// The page's side of the spreadsheet worker (sheetWorker.ts): open a file,
// ask for one sheet at a time, and give up on a parse that takes too long.

export interface SheetCell { text: string; numeric: boolean }

export interface SheetData {
  rows: SheetCell[][]
  headers: string[]
  totalRows: number
  totalCols: number
}

export type SheetRequest =
  | { id: number; type: 'open'; bytes: Uint8Array; maxRows: number }
  | { id: number; type: 'sheet'; name: string; maxRows: number; maxCols: number }

export type SheetReply =
  | { id: number; ok: true; names?: string[]; sheet?: SheetData }
  | { id: number; ok: false; error: string }

// A real workbook under the preview size limit opens in a second or two. One
// still parsing after this long is treated as broken, and its worker ended.
const SHEET_TIMEOUT_MS = 30_000

export class SheetPreview {
  private worker: Worker | null
  private nextId = 1
  private pending = new Map<number, { resolve: (r: SheetReply & { ok: true }) => void; reject: (e: Error) => void }>()

  constructor(private maxRows: number, private maxCols: number) {
    this.worker = new Worker(new URL('./sheetWorker.ts', import.meta.url), { type: 'module' })
    this.worker.onmessage = (e: MessageEvent<SheetReply>) => {
      const p = this.pending.get(e.data.id)
      if (!p) return
      this.pending.delete(e.data.id)
      if (e.data.ok) p.resolve(e.data)
      else p.reject(new Error(e.data.error))
    }
    this.worker.onerror = (e) => this.fail(new Error(e.message || 'spreadsheet worker failed'))
  }

  private ask(req: Record<string, unknown> & { type: SheetRequest['type'] }, transfer: Transferable[] = []) {
    return new Promise<SheetReply & { ok: true }>((resolve, reject) => {
      if (!this.worker) {
        reject(new Error('spreadsheet preview closed'))
        return
      }
      const id = this.nextId++
      const timer = setTimeout(() => this.fail(new Error('spreadsheet took too long to open')), SHEET_TIMEOUT_MS)
      this.pending.set(id, {
        resolve: (r) => { clearTimeout(timer); resolve(r) },
        reject: (e) => { clearTimeout(timer); reject(e) },
      })
      this.worker.postMessage({ ...req, id }, transfer)
    })
  }

  /** Parses the file and returns its sheet names. */
  async open(bytes: Uint8Array): Promise<string[]> {
    const r = await this.ask({ type: 'open', bytes, maxRows: this.maxRows }, [bytes.buffer])
    return r.names ?? []
  }

  /** The visible part of one sheet, already formatted as text. */
  async sheet(name: string): Promise<SheetData> {
    const r = await this.ask({ type: 'sheet', name, maxRows: this.maxRows, maxCols: this.maxCols })
    return r.sheet ?? { rows: [], headers: [], totalRows: 0, totalCols: 0 }
  }

  /** Ends the worker; anything still waiting fails. */
  close() {
    this.fail(new Error('spreadsheet preview closed'))
  }

  private fail(err: Error) {
    this.worker?.terminate()
    this.worker = null
    for (const p of this.pending.values()) p.reject(err)
    this.pending.clear()
  }
}
