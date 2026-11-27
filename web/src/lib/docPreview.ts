// Office and Hangul documents previewed in the browser. Each kind's renderer
// is a lazily imported chunk (rhwp alone is a ~10 MB gzipped WASM module), so
// none of this costs anything until someone opens a preview.
export type DocPreviewKind = 'hwp' | 'docx' | 'sheet'

// Bigger documents are a download: parsing happens on the main thread.
export const DOC_PREVIEW_MAX_BYTES = 50 * 1024 * 1024

const BY_EXT: Record<string, DocPreviewKind> = {
  hwp: 'hwp',
  hwpx: 'hwp',
  docx: 'docx',
  xlsx: 'sheet',
  xlsm: 'sheet',
  xls: 'sheet',
  ods: 'sheet',
}

const BY_TYPE: Record<string, DocPreviewKind> = {
  'application/x-hwp': 'hwp',
  'application/haansofthwp': 'hwp',
  'application/vnd.hancom.hwp': 'hwp',
  'application/vnd.hancom.hwpx': 'hwp',
  'application/hwp+zip': 'hwp',
  'application/vnd.openxmlformats-officedocument.wordprocessingml.document': 'docx',
  'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': 'sheet',
  'application/vnd.ms-excel.sheet.macroenabled.12': 'sheet',
  'application/vnd.ms-excel': 'sheet',
  'application/vnd.oasis.opendocument.spreadsheet': 'sheet',
}

export function docPreviewKind(filename: string | undefined, contentType: string | undefined, size: number): DocPreviewKind | null {
  if (size > DOC_PREVIEW_MAX_BYTES) return null
  const ct = (contentType ?? '').toLowerCase().split(';')[0].trim()
  const name = (filename ?? '').toLowerCase()
  const ext = name.includes('.') ? name.split('.').pop()! : ''
  // The extension wins: mailers often send these as application/octet-stream
  // or a generic zip type, and a wrong guess only means a failed preview.
  return BY_EXT[ext] ?? BY_TYPE[ct] ?? null
}
