/// <reference lib="webworker" />
// Spreadsheet attachments are parsed here, in a worker, not on the page. The
// file comes from whoever sent the email: one built to make the parser spin
// would otherwise freeze the whole app. Here it can only stall this worker,
// which the page gives up on and ends (see DocumentPreview.vue).
import * as XLSX from 'xlsx'
import * as cptable from 'xlsx/dist/cpexcel.full.mjs'
import type { SheetData, SheetReply, SheetRequest } from './sheetPreview'

// Legacy .xls files in Korean (CP949) and other code pages need the tables.
XLSX.set_cptable(cptable)

let workbook: XLSX.WorkBook | null = null

function sheetData(name: string, maxRows: number, maxCols: number): SheetData {
  const ws = workbook?.Sheets[name]
  if (!ws) return { rows: [], headers: [], totalRows: 0, totalCols: 0 }
  const full = XLSX.utils.decode_range(ws['!fullref'] ?? ws['!ref'] ?? 'A1')
  const used = XLSX.utils.decode_range(ws['!ref'] ?? 'A1')
  const totalRows = full.e.r + 1
  const totalCols = full.e.c + 1
  const rowCount = Math.min(used.e.r + 1, maxRows)
  const cols = Math.min(totalCols, maxCols)
  // Read with dense: true, so cells are rows of arrays. format_cell applies
  // the sheet's number format (thousands separators, dates, percentages).
  const data = (ws['!data'] ?? []) as (XLSX.CellObject | undefined)[][]
  const rows: SheetData['rows'] = []
  for (let r = 0; r < rowCount; r++) {
    const src = data[r] ?? []
    const row: SheetData['rows'][number] = []
    for (let c = 0; c < cols; c++) {
      const cell = src[c]
      row.push(cell ? { text: XLSX.utils.format_cell(cell), numeric: cell.t === 'n' } : { text: '', numeric: false })
    }
    rows.push(row)
  }
  return {
    rows,
    headers: Array.from({ length: cols }, (_, i) => XLSX.utils.encode_col(i)),
    totalRows,
    totalCols,
  }
}

self.onmessage = (e: MessageEvent<SheetRequest>) => {
  const req = e.data
  let reply: SheetReply
  try {
    if (req.type === 'open') {
      workbook = XLSX.read(req.bytes, { type: 'array', dense: true, sheetRows: req.maxRows + 1, cellHTML: false })
      reply = { id: req.id, ok: true, names: workbook.SheetNames }
    } else {
      reply = { id: req.id, ok: true, sheet: sheetData(req.name, req.maxRows, req.maxCols) }
    }
  } catch (err) {
    reply = { id: req.id, ok: false, error: String(err) }
  }
  self.postMessage(reply)
}
