import { ref, computed } from 'vue'
import { api, quarantined, type ImportStatus } from '@/lib/api'
import { locked } from '@/lib/auth'
import { senderName } from '@/lib/format'
import { i18n } from '@/i18n'
import { router } from '@/router'
import { onMailSignal, startMailEvents } from '@/composables/useMailEvents'

// Opt-in desktop notifications for mail a background IMAP sync brought in.
// The choice is kept per browser, because the permission it depends on is
// per browser too. There is no service worker: notifications come from an
// open Local Eml tab. The server's live mail stream says when a sync added
// mail; the tab then reads which jobs finished since it last looked. A slow
// poll covers a stream that is down.

const STORAGE_KEY = 'notify-new-mail'
const FALLBACK_POLL_MS = 5 * 60_000
// Matches the server's job kind for background syncs (kindIMAPSync).
const SYNC_KIND = 'imap-sync'

export type NotifyPermission = NotificationPermission | 'unsupported'

function supported(): boolean {
  return typeof window !== 'undefined' && 'Notification' in window
}

function readPermission(): NotifyPermission {
  return supported() ? Notification.permission : 'unsupported'
}

function readPref(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1'
  } catch {
    return false
  }
}

function writePref(on: boolean) {
  try {
    if (on) localStorage.setItem(STORAGE_KEY, '1')
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    /* storage unavailable: the choice lasts for this page only */
  }
}

const wanted = ref(readPref())
const permission = ref<NotifyPermission>(readPermission())
/** On, and the browser lets us show notifications. */
const active = computed(() => wanted.value && permission.value === 'granted')

export type EnableResult = 'on' | 'off' | 'denied' | 'unsupported'

/**
 * Turns notifications on or off. Turning on asks the browser for permission
 * the first time; if the person declines, or blocked it earlier, the option
 * stays off and the caller explains how to allow it.
 */
async function setEnabled(on: boolean): Promise<EnableResult> {
  if (!on) {
    wanted.value = false
    writePref(false)
    return 'off'
  }
  if (!supported()) {
    permission.value = 'unsupported'
    return 'unsupported'
  }
  if (Notification.permission === 'default') {
    try {
      await Notification.requestPermission()
    } catch {
      /* treated as not granted below */
    }
  }
  permission.value = Notification.permission
  if (permission.value !== 'granted') {
    wanted.value = false
    writePref(false)
    return 'denied'
  }
  wanted.value = true
  writePref(true)
  resetCursor()
  show(t('notify.enabled_title'), t('notify.enabled_body'), 'local-eml-notify-on')
  return 'on'
}

function t(key: string, params?: Record<string, unknown>, plural?: number): string {
  return plural == null ? i18n.global.t(key, params ?? {}) : i18n.global.t(key, params ?? {}, plural)
}

function show(title: string, body: string, tag: string, onClick?: () => void) {
  try {
    const n = new Notification(title, { body, tag, icon: '/icon-64.png' })
    n.onclick = () => {
      window.focus()
      onClick?.()
      n.close()
    }
  } catch {
    // Some browsers only allow notifications from a service worker; there is
    // nothing useful to do here then.
  }
}

// --- watcher ---

const nowS = () => Math.floor(Date.now() / 1000)
let cursor = nowS()
// Job ids already handled: `since` is inclusive, so the job(s) finishing in
// the cursor's own second come back on the next poll.
let seen = new Set<string>()
let timer: number | null = null

function resetCursor() {
  cursor = nowS()
  seen = new Set()
}

const added = (j: ImportStatus) => Math.max(0, j.processed - j.duplicates - j.errors)
const unix = (iso: string) => Math.floor(new Date(iso).getTime() / 1000)

// One check at a time: a pushed event and the fallback tick landing together
// must not announce the same jobs twice.
let checking: Promise<void> | null = null
let checkAgain = false

function poll(): Promise<void> {
  if (checking) {
    checkAgain = true
    return checking
  }
  checking = check().finally(() => {
    checking = null
    if (checkAgain) {
      checkAgain = false
      void poll()
    }
  })
  return checking
}

async function check() {
  permission.value = readPermission()
  if (!active.value || locked.value) {
    // Nothing is announced while off, and turning back on starts from now
    // rather than replaying what came in meanwhile.
    resetCursor()
    return
  }
  let jobs: ImportStatus[]
  try {
    jobs = await api.listFinishedImports(SYNC_KIND, cursor)
  } catch {
    return // offline or locked: the app shows that already; try again later
  }
  const fresh = jobs.filter((j) => !seen.has(j.id))
  if (jobs.length) {
    cursor = Math.max(cursor, ...jobs.map((j) => unix(j.finished_at)))
    seen = new Set(jobs.filter((j) => unix(j.finished_at) >= cursor).map((j) => j.id))
  }
  const count = fresh.reduce((n, j) => n + added(j), 0)
  if (count === 0 || !active.value) return
  const firstStart = Math.min(...fresh.filter((j) => added(j) > 0).map((j) => unix(j.started_at)))
  await notifyNewMail(count, firstStart, fresh[fresh.length - 1].id)
}

async function notifyNewMail(count: number, sinceS: number, tagID: string) {
  // The newest message names the notification, as mail clients do. It is
  // only used if it really came in with this sync and isn't flagged spam or
  // phishing, whose subject shouldn't pop up on screen.
  let latest = null
  try {
    const list = await api.listEmails({ sort: 'imported_at', order: 'desc', limit: 1 })
    const e = list.items[0]
    if (e && unix(e.imported_at) >= sinceS && !quarantined(e.flag)) latest = e
  } catch {
    /* fall back to the count alone */
  }
  const subject = latest?.subject || t('notify.no_subject')
  const from = latest ? senderName(latest.from) : ''
  const tag = `local-eml-new-mail-${tagID}`

  if (count === 1 && latest) {
    const sha = latest.sha256
    show(from || t('notify.count_title', { n: 1 }, 1), subject, tag, () =>
      router.push({ name: 'viewer', params: { sha } }),
    )
    return
  }
  const title = t('notify.count_title', { n: count }, count)
  const body = latest ? t('notify.latest_body', { from, subject }) : t('notify.generic_body')
  show(title, body, tag, () => router.push({ name: 'library' }))
}

/** Starts watching once for the page. Safe to call more than once. */
export function startNewMailWatcher() {
  if (timer != null) return
  startMailEvents()
  onMailSignal((sig) => {
    if (sig.type === 'resumed' || sig.kind === SYNC_KIND) void poll()
  })
  timer = window.setInterval(() => void poll(), FALLBACK_POLL_MS)
  // A permission changed in the browser's site settings shows up when the
  // person comes back to the tab.
  document.addEventListener('visibilitychange', () => {
    permission.value = readPermission()
  })
}

export function useNewMailNotifier() {
  return { wanted, permission, active, setEnabled }
}
