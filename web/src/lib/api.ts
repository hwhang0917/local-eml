import { locked } from '@/lib/auth'
export type EmailFlag = '' | 'spam' | 'phishing' | 'safe'
// Spam and phishing hide the message and strip it to text; safe only mutes
// the phishing heuristic.
export const quarantined = (flag: EmailFlag) => flag === 'spam' || flag === 'phishing'

export interface RiskReason {
  code: string
  detail?: string
}

export interface Email {
  id: number
  sha256: string
  filename: string
  subject: string
  from: string
  to: string[]
  cc: string[]
  message_id: string
  /** Conversation key; absent when the message has no Message-ID at all. */
  thread_id?: string
  /** Only in grouped listings: number of messages in this row's conversation. */
  thread_count?: number
  sent_at: string
  received_at: string
  size_bytes: number
  has_attachments: boolean
  attachment_count: number
  imported_at: string
  starred: boolean
  /** '' or 'spam' | 'phishing'. Flagged mail is hidden and plain-text only. */
  flag: EmailFlag
  /** Phishing heuristics' findings; empty when clean, null when not yet assessed. */
  risk: RiskReason[] | null
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

export interface EmailListParams {
  q?: string
  starred?: boolean
  flagged?: boolean
  category?: string
  from?: string
  to?: string
  sort?: string
  order?: 'asc' | 'desc'
  limit?: number
  offset?: number
  group?: 'thread'
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

/** Response of every endpoint that queues a background job. */
export interface JobStarted {
  import_id: string
  kind: string
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

export interface Stats {
  total_count: number
  total_bytes: number
  starred_count: number
  attachment_count: number
  undated_count: number
  per_year: { year: string; count: number }[]
  top_senders: { from: string; count: number }[]
  per_category: { category_id: number; count: number }[]
}

// service: started by the service manager. temporary: throwaway folder,
// deleted on exit. normal: double-click or terminal run on the real library.
export type SessionMode = 'service' | 'temporary' | 'normal'

export interface SessionInfo {
  mode: SessionMode
  temporary_launches: boolean
}

export interface RestoreSummary {
  emails: number
  categories: number
  settings: number
  imap_profiles: number
  s3_profiles: number
}

async function okOrThrow(res: Response): Promise<void> {
  if (res.status === 401) locked.value = true
  if (!res.ok) {
    const text = await res.text().catch(() => '')
    throw new Error(`${res.status} ${res.statusText}: ${text}`)
  }
}

async function jsonOrThrow<T>(res: Response): Promise<T> {
  await okOrThrow(res)
  return res.json()
}

async function textOrThrow(res: Response): Promise<string> {
  await okOrThrow(res)
  return res.text()
}

function jsonBody(method: 'POST' | 'PUT', body: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  }
}

export const api = {
  getSession() {
    return fetch('/api/session').then(jsonOrThrow<SessionInfo>)
  },

  setTemporaryLaunches(enabled: boolean) {
    return fetch('/api/session/temporary-launches', jsonBody('PUT', { enabled })).then(okOrThrow)
  },

  wipeData() {
    return fetch('/api/data/wipe', { method: 'POST' }).then(jsonOrThrow<{ emails: number }>)
  },

  listEmails(params: EmailListParams = {}) {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v === undefined || v === '' || v === false) continue
      qs.set(k, v === true ? '1' : String(v))
    }
    return fetch(`/api/emails?${qs}`).then(jsonOrThrow<EmailList>)
  },

  setStarred(sha: string, starred: boolean) {
    return fetch(`/api/emails/${sha}/star`, { method: starred ? 'PUT' : 'DELETE' }).then(okOrThrow)
  },

  /** Empty flag clears it. */
  setFlag(sha: string, flag: EmailFlag) {
    const url = `/api/emails/${sha}/flag`
    return fetch(url, flag ? jsonBody('PUT', { flag }) : { method: 'DELETE' }).then(okOrThrow)
  },

  getStats() {
    return fetch('/api/stats').then(jsonOrThrow<Stats>)
  },

  listCategories() {
    return fetch('/api/categories').then(jsonOrThrow<Category[]>)
  },

  /** The only write: the palette is fixed, so a category can only be renamed. */
  renameCategory(id: number, name: string) {
    return fetch(`/api/categories/${id}`, jsonBody('PUT', { name })).then(jsonOrThrow<Category>)
  },

  setCategory(sha: string, categoryId: number | null) {
    const url = categoryId === null
      ? `/api/emails/${sha}/category`
      : `/api/emails/${sha}/category/${categoryId}`
    return fetch(url, { method: categoryId === null ? 'DELETE' : 'PUT' }).then(okOrThrow)
  },

  getEmail(sha: string) {
    return fetch(`/api/emails/${sha}`).then(jsonOrThrow<Email>)
  },

  getThread(sha: string) {
    return fetch(`/api/emails/${sha}/thread`).then(jsonOrThrow<{ items: Email[] }>)
  },

  deleteEmail(sha: string) {
    return fetch(`/api/emails/${sha}`, { method: 'DELETE' }).then(okOrThrow)
  },

  indexEmail(sha: string) {
    return fetch(`/api/emails/${sha}/index`, { method: 'POST' })
      .then(jsonOrThrow<{ sha256: string; email_id: number; duplicate: boolean }>)
  },

  getParts(sha: string) {
    return fetch(`/api/emails/${sha}/parts`).then(jsonOrThrow<PartsManifest>)
  },

  htmlURL: (sha: string, remote = false, lang = 'en') =>
    `/api/emails/${sha}/html?lang=${encodeURIComponent(lang)}${remote ? '&remote=1' : ''}`,
  /** inline: serve for display in the page rather than as a download. */
  attachmentURL: (sha: string, idx: number, inline = false) =>
    `/api/emails/${sha}/attachments/${idx}${inline ? '?inline=1' : ''}`,

  getText(sha: string) {
    return fetch(`/api/emails/${sha}/text`).then(textOrThrow)
  },

  getRaw(sha: string) {
    return fetch(`/api/emails/${sha}/raw`).then(textOrThrow)
  },

  getAttachmentText(sha: string, idx: number) {
    return fetch(api.attachmentURL(sha, idx, true)).then(textOrThrow)
  },

  upload(files: File[]) {
    const fd = new FormData()
    for (const f of files) fd.append('file', f, f.name)
    return fetch('/api/imports', { method: 'POST', body: fd }).then(jsonOrThrow<JobStarted>)
  },

  uploadS3(cfg: S3ImportConfig) {
    return fetch('/api/imports/s3', jsonBody('POST', cfg)).then(jsonOrThrow<JobStarted>)
  },

  uploadImap(cfg: IMAPImportConfig) {
    return fetch('/api/imports/imap', jsonBody('POST', cfg)).then(jsonOrThrow<JobStarted>)
  },

  resync() {
    return fetch('/api/imports/resync', { method: 'POST' }).then(jsonOrThrow<JobStarted & { total: number }>)
  },

  importStatus(id: string) {
    return fetch(`/api/imports/${id}`).then(jsonOrThrow<ImportStatus>)
  },

  importEventSource(id: string): EventSource {
    return new EventSource(`/api/imports/${id}/events`)
  },

  listIMAPProfiles() {
    return fetch('/api/imap/profiles').then(jsonOrThrow<IMAPProfile[]>)
  },

  saveIMAPProfile(p: Pick<IMAPProfile, 'name' | 'host' | 'port' | 'username' | 'folder'> & { sync_enabled?: boolean }) {
    return fetch('/api/imap/profiles', jsonBody('POST', p)).then(jsonOrThrow<IMAPProfile>)
  },

  deleteIMAPProfile(id: number) {
    return fetch(`/api/imap/profiles/${id}`, { method: 'DELETE' }).then(okOrThrow)
  },

  getIMAPSyncInterval() {
    return fetch('/api/imap/sync-interval').then(jsonOrThrow<{ seconds: number }>)
  },

  setIMAPSyncInterval(seconds: number) {
    return fetch('/api/imap/sync-interval', jsonBody('PUT', { seconds })).then(okOrThrow)
  },

  syncIMAPProfile(id: number) {
    return fetch(`/api/imap/profiles/${id}/sync`, { method: 'POST' }).then(okOrThrow)
  },

  listS3Profiles() {
    return fetch('/api/s3/profiles').then(jsonOrThrow<S3Profile[]>)
  },

  saveS3Profile(p: Omit<S3Profile, 'id'>) {
    return fetch('/api/s3/profiles', jsonBody('POST', p)).then(jsonOrThrow<S3Profile>)
  },

  deleteS3Profile(id: number) {
    return fetch(`/api/s3/profiles/${id}`, { method: 'DELETE' }).then(okOrThrow)
  },

  exportZipURL: () => '/api/exports/zip',

  restoreBackup(file: File) {
    const form = new FormData()
    form.append('file', file)
    return fetch('/api/restore', { method: 'POST', body: form }).then(jsonOrThrow<RestoreSummary>)
  },

  exportS3(cfg: S3ImportConfig) {
    return fetch('/api/exports/s3', jsonBody('POST', cfg)).then(jsonOrThrow<{ export_id: string; kind: string }>)
  },

  checkUpdate(force = false) {
    return fetch(`/api/update/check${force ? '?force=1' : ''}`).then(jsonOrThrow<UpdateStatus>)
  },

  installUpdate() {
    return fetch('/api/update/install', { method: 'POST' })
      .then(jsonOrThrow<{ status: string; from: string; to: string; restarting: boolean }>)
  },

  async cancelJob(id: string): Promise<void> {
    const res = await fetch(`/api/jobs/${id}`, { method: 'DELETE' })
    // Already finished and forgotten: nothing left to cancel, not an error.
    if (res.status === 404) return
    await okOrThrow(res)
  },
}
