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
  starred: boolean
  /** Row exists but its .eml is gone from disk. */
  blob_missing?: boolean
  /** .eml is on disk but no row exists; metadata was parsed from the file. */
  not_indexed?: boolean
  /** Absent when uncategorized. Resolve against the cached category list. */
  category_id?: number
}

export interface Category {
  id: number
  color: string
  /** Empty until renamed; the UI then shows the colour's own name. */
  name: string
  position: number
}

export interface EmailList {
  total: number
  limit: number
  offset: number
  items: Email[]
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
  profile_id?: number
  host: string
  port?: number
  username: string
  password: string
  folder?: string
}

export interface UpdateStatus {
  current: string
  latest?: string
  has_update: boolean
  release_url?: string
  notes?: string
  asset_name?: string
  can_install: boolean
  install_note?: string
  error?: string
}

export interface IMAPProfile {
  id: number
  name: string
  host: string
  port?: number
  username: string
  folder?: string
  sync_enabled: boolean
  uid_validity?: number
  last_uid?: number
  last_synced_at?: number
  has_password: boolean
}

export interface S3Profile {
  id: number
  name: string
  bucket: string
  prefix?: string
  region?: string
  access_key_id?: string
}

const BASE = ''

async function jsonOrThrow<T>(res: Response): Promise<T> {
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`${res.status} ${res.statusText}: ${text}`)
  }
  return (await res.json()) as T
}

async function okOrThrow(res: Response): Promise<void> {
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`${res.status} ${res.statusText}: ${text}`)
  }
}

export const api = {
  listEmails(params: { q?: string; starred?: boolean; category?: string; from?: string; to?: string; sort?: string; order?: 'asc' | 'desc'; limit?: number; offset?: number } = {}) {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v === undefined || v === '' || v === false) continue
      qs.set(k, v === true ? '1' : String(v))
    }
    return fetch(`${BASE}/api/emails?${qs}`).then(jsonOrThrow<EmailList>)
  },

  async setStarred(sha: string, starred: boolean): Promise<void> {
    const res = await fetch(`${BASE}/api/emails/${sha}/star`, {
      method: starred ? 'PUT' : 'DELETE',
    })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
  },

  listCategories() {
    return fetch(`${BASE}/api/categories`).then(jsonOrThrow<Category[]>)
  },

  /** The only write: the palette is fixed, so a category can only be renamed. */
  renameCategory(id: number, name: string) {
    return fetch(`${BASE}/api/categories/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    }).then(jsonOrThrow<Category>)
  },

  setCategory(sha: string, categoryId: number | null) {
    const url = categoryId === null
      ? `${BASE}/api/emails/${sha}/category`
      : `${BASE}/api/emails/${sha}/category/${categoryId}`
    return fetch(url, { method: categoryId === null ? 'DELETE' : 'PUT' }).then(okOrThrow)
  },

  getEmail(sha: string) {
    return fetch(`${BASE}/api/emails/${sha}`).then(jsonOrThrow<Email>)
  },

  deleteEmail(sha: string) {
    return fetch(`${BASE}/api/emails/${sha}`, { method: 'DELETE' }).then(okOrThrow)
  },

  indexEmail(sha: string) {
    return fetch(`${BASE}/api/emails/${sha}/index`, { method: 'POST' })
      .then(jsonOrThrow<{ sha256: string; email_id: number; duplicate: boolean }>)
  },

  getParts(sha: string) {
    return fetch(`${BASE}/api/emails/${sha}/parts`).then(jsonOrThrow<PartsManifest>)
  },

  textURL: (sha: string) => `${BASE}/api/emails/${sha}/text`,
  rawURL: (sha: string) => `${BASE}/api/emails/${sha}/raw`,
  htmlURL: (sha: string, remote = false, lang = 'en') =>
    `${BASE}/api/emails/${sha}/html?lang=${encodeURIComponent(lang)}${remote ? '&remote=1' : ''}`,
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

  async resync(): Promise<{ import_id: string; kind: string; total: number }> {
    const res = await fetch(`${BASE}/api/imports/resync`, { method: 'POST' })
    return jsonOrThrow(res)
  },

  importStatus(id: string) {
    return fetch(`${BASE}/api/imports/${id}`).then(jsonOrThrow<ImportStatus>)
  },

  importEventSource(id: string): EventSource {
    return new EventSource(`${BASE}/api/imports/${id}/events`)
  },

  listIMAPProfiles() {
    return fetch(`${BASE}/api/imap/profiles`).then(jsonOrThrow<IMAPProfile[]>)
  },

  async saveIMAPProfile(p: {
    name: string
    host: string
    port?: number
    username: string
    folder?: string
    sync_enabled?: boolean
  }): Promise<IMAPProfile> {
    const res = await fetch(`${BASE}/api/imap/profiles`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(p),
    })
    return jsonOrThrow<IMAPProfile>(res)
  },

  async deleteIMAPProfile(id: number): Promise<void> {
    const res = await fetch(`${BASE}/api/imap/profiles/${id}`, { method: 'DELETE' })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
  },

  async syncIMAPProfile(id: number): Promise<void> {
    const res = await fetch(`${BASE}/api/imap/profiles/${id}/sync`, { method: 'POST' })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
  },

  listS3Profiles() {
    return fetch(`${BASE}/api/s3/profiles`).then(jsonOrThrow<S3Profile[]>)
  },

  async saveS3Profile(p: Omit<S3Profile, 'id'>): Promise<S3Profile> {
    const res = await fetch(`${BASE}/api/s3/profiles`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(p),
    })
    return jsonOrThrow<S3Profile>(res)
  },

  async deleteS3Profile(id: number): Promise<void> {
    const res = await fetch(`${BASE}/api/s3/profiles/${id}`, { method: 'DELETE' })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
  },

  exportZipURL: () => `${BASE}/api/exports/zip`,

  async exportS3(cfg: S3ImportConfig): Promise<{ export_id: string; kind: string }> {
    const res = await fetch(`${BASE}/api/exports/s3`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    })
    return jsonOrThrow(res)
  },

  async checkUpdate(force = false): Promise<UpdateStatus> {
    const url = `${BASE}/api/update/check${force ? '?force=1' : ''}`
    return fetch(url).then(jsonOrThrow<UpdateStatus>)
  },

  async installUpdate(): Promise<{ status: string; from: string; to: string; restarting: boolean }> {
    const res = await fetch(`${BASE}/api/update/install`, { method: 'POST' })
    if (!res.ok) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
    return res.json()
  },

  async cancelJob(id: string): Promise<void> {
    const res = await fetch(`${BASE}/api/jobs/${id}`, { method: 'DELETE' })
    if (!res.ok && res.status !== 404) {
      const text = await res.text().catch(() => '')
      throw new Error(`${res.status} ${res.statusText}: ${text}`)
    }
  },
}
