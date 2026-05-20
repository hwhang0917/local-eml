export interface Email {
  id: number
  sha256: string
  filename: string
  subject: string
  from: string
  to: string[]
  cc: string[]
  message_id: string
  sent_at: string
  received_at: string
  size_bytes: number
  has_attachments: boolean
  attachment_count: number
  imported_at: string
  tags: string[]
}

export interface EmailList {
  total: number
  limit: number
  offset: number
  items: Email[]
}

export interface Tag {
  name: string
  count: number
}

export interface PartInfo {
  index: number
  content_id?: string
  content_type: string
  filename?: string
  size: number
}

export interface PartsManifest {
  has_text: boolean
  has_html: boolean
  inlines: PartInfo[]
  attachments: PartInfo[]
}

export interface ImportStatus {
  id: string
  source_kind: string
  source_name: string
  status: 'queued' | 'running' | 'done' | 'error'
  total: number
  processed: number
  duplicates: number
  errors: number
  started_at: string
  finished_at: string
}

export interface ImportEvent {
  type: 'step' | 'item' | 'done' | 'error'
  phase?: string
  path?: string
  sha256?: string
  duplicate?: boolean
  message?: string
  processed: number
  total: number
}

export interface S3ImportConfig {
  accessKeyId?: string
  secretAccessKey?: string
  sessionToken?: string
  region?: string
  bucket: string
  prefix?: string
}

export interface IMAPImportConfig {
  host: string
  port?: number
  username: string
  password: string
  folder?: string
}

const BASE = ''

async function jsonOrThrow<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`${res.status} ${res.statusText}: ${text}`)
  }
  return (await res.json()) as T
}

export const api = {
  listEmails(params: { q?: string; tag?: string; sort?: string; order?: 'asc' | 'desc'; limit?: number; offset?: number } = {}) {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== '') qs.set(k, String(v))
    }
    return fetch(`${BASE}/api/emails?${qs}`).then(jsonOrThrow<EmailList>)
  },

  getEmail(sha: string) {
    return fetch(`${BASE}/api/emails/${sha}`).then(jsonOrThrow<Email>)
  },

  getParts(sha: string) {
    return fetch(`${BASE}/api/emails/${sha}/parts`).then(jsonOrThrow<PartsManifest>)
  },

  textURL: (sha: string) => `${BASE}/api/emails/${sha}/text`,
  rawURL: (sha: string) => `${BASE}/api/emails/${sha}/raw`,
  htmlURL: (sha: string, remote = false) => `${BASE}/api/emails/${sha}/html${remote ? '?remote=1' : ''}`,
  attachmentURL: (sha: string, idx: number) => `${BASE}/api/emails/${sha}/attachments/${idx}`,

  async getText(sha: string): Promise<string> {
    const res = await fetch(api.textURL(sha))
    if (!res.ok) throw new Error(`${res.status}`)
    return res.text()
  },

  async getRaw(sha: string): Promise<string> {
    const res = await fetch(api.rawURL(sha))
    if (!res.ok) throw new Error(`${res.status}`)
    return res.text()
  },

  listTags() {
    return fetch(`${BASE}/api/tags`).then(jsonOrThrow<Tag[]>)
  },

  async addTag(sha: string, name: string): Promise<void> {
    const res = await fetch(`${BASE}/api/emails/${sha}/tags`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    })
    if (!res.ok) throw new Error(await res.text())
  },

  async removeTag(sha: string, name: string): Promise<void> {
    const res = await fetch(`${BASE}/api/emails/${sha}/tags/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    })
    if (!res.ok) throw new Error(await res.text())
  },

  async upload(files: File[]): Promise<{ import_id: string; kind: string }> {
    const fd = new FormData()
    for (const f of files) fd.append('file', f, f.name)
    const res = await fetch(`${BASE}/api/imports`, { method: 'POST', body: fd })
    return jsonOrThrow(res)
  },

  async uploadS3(cfg: S3ImportConfig): Promise<{ import_id: string; kind: string }> {
    const res = await fetch(`${BASE}/api/imports/s3`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    })
    return jsonOrThrow(res)
  },

  async uploadImap(cfg: IMAPImportConfig): Promise<{ import_id: string; kind: string }> {
    const res = await fetch(`${BASE}/api/imports/imap`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    })
    return jsonOrThrow(res)
  },

  importStatus(id: string) {
    return fetch(`${BASE}/api/imports/${id}`).then(jsonOrThrow<ImportStatus>)
  },

  importEventSource(id: string): EventSource {
    return new EventSource(`${BASE}/api/imports/${id}/events`)
  },
}
