import { ref, onMounted, onUnmounted } from 'vue'
import { APP_VERSION } from '@/version'

const ONLINE_INTERVAL_MS = 30_000
const OFFLINE_INTERVAL_MS = 3_000
const PROBE_TIMEOUT_MS = 4_000
// How long to keep polling fast after an update was triggered, so the reload
// lands seconds after the new binary comes up rather than half a minute later.
const RESTART_WATCH_MS = 5 * 60_000
const RELOADED_KEY = 'reloaded-for-version'

const online = ref(true)
const lastChecked = ref<Date | null>(null)
const checking = ref(false)

let timer: number | null = null
let subscribers = 0
let watchRestartUntil = 0

// A binary swap leaves this page running assets from the previous version.
// Reload once when the server reports a different version. Skipped in dev
// (Vite serves its own assets) and guarded so a stale cache can't loop.
function reloadIfServerChanged(serverVersion: unknown) {
  if (import.meta.env.DEV || typeof serverVersion !== 'string' || !serverVersion) return
  if (serverVersion === APP_VERSION || serverVersion === 'dev') return
  try {
    if (sessionStorage.getItem(RELOADED_KEY) === serverVersion) return
    sessionStorage.setItem(RELOADED_KEY, serverVersion)
  } catch {
    /* storage unavailable: still reload once per page life */
  }
  window.location.reload()
}

async function probe() {
  if (checking.value) return
  checking.value = true
  const ctrl = new AbortController()
  const t = window.setTimeout(() => ctrl.abort(), PROBE_TIMEOUT_MS)
  try {
    const res = await fetch('/healthz', { cache: 'no-store', signal: ctrl.signal })
    online.value = res.ok
    if (res.ok) reloadIfServerChanged((await res.json().catch(() => null))?.version)
  } catch {
    online.value = false
  } finally {
    window.clearTimeout(t)
    lastChecked.value = new Date()
    checking.value = false
    schedule()
  }
}

function schedule() {
  if (timer != null) window.clearTimeout(timer)
  if (subscribers === 0) return
  const fast = !online.value || Date.now() < watchRestartUntil
  timer = window.setTimeout(probe, fast ? OFFLINE_INTERVAL_MS : ONLINE_INTERVAL_MS)
}

/** Call after triggering a restart: polls fast until the new server answers. */
export function expectRestart() {
  watchRestartUntil = Date.now() + RESTART_WATCH_MS
  schedule()
}

export function useHealth() {
  onMounted(() => {
    subscribers++
    if (subscribers === 1) probe()
  })
  onUnmounted(() => {
    subscribers = Math.max(0, subscribers - 1)
    if (subscribers === 0 && timer != null) {
      window.clearTimeout(timer)
      timer = null
    }
  })
  return { online, lastChecked, checking, retry: probe }
}
