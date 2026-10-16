import { ref } from 'vue'
import { api, type ImportStatus } from '@/lib/api'

// Module scope so the running flag and the poll loop survive route changes —
// leaving the settings page must not orphan a rescan in progress.
// ponytail: a full page reload forgets a running job; add a server-side
// "active resync" query if that ever matters.
const running = ref(false)
const POLL_MS = 1000

export function useResync() {
  // Starts the rescan and polls it to completion. Returns the final job
  // status, or null when a rescan was already running.
  async function start(): Promise<ImportStatus | null> {
    if (running.value) return null
    running.value = true
    try {
      const { import_id } = await api.resync()
      for (;;) {
        await new Promise((r) => setTimeout(r, POLL_MS))
        const st = await api.importStatus(import_id)
        if (st.status === 'done' || st.status === 'error') return st
      }
    } finally {
      running.value = false
    }
  }

  return { running, start }
}
