import { ref, onMounted, onUnmounted } from 'vue'

const ONLINE_INTERVAL_MS = 30_000
const OFFLINE_INTERVAL_MS = 3_000
const PROBE_TIMEOUT_MS = 4_000

const online = ref(true)
const lastChecked = ref<Date | null>(null)
const checking = ref(false)

let timer: number | null = null
let subscribers = 0

async function probe() {
  if (checking.value) return
  checking.value = true
  const ctrl = new AbortController()
  const t = window.setTimeout(() => ctrl.abort(), PROBE_TIMEOUT_MS)
  try {
    const res = await fetch('/healthz', { cache: 'no-store', signal: ctrl.signal })
    online.value = res.ok
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
  const delay = online.value ? ONLINE_INTERVAL_MS : OFFLINE_INTERVAL_MS
  timer = window.setTimeout(probe, delay)
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
