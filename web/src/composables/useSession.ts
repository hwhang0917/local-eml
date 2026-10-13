import { ref } from 'vue'
import { api, type SessionInfo } from '@/lib/api'

// One fetch per page life, shared by the banner and the Privacy page. The mode
// can't change without a restart, which reloads the page anyway.
const session = ref<SessionInfo | null>(null)
let loaded: Promise<void> | null = null

async function refresh() {
  try {
    session.value = await api.getSession()
  } catch {
    // The offline banner already covers an unreachable server.
  }
}

export function useSession() {
  if (!loaded) loaded = refresh()
  return { session, refresh }
}
