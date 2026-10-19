import { computed, ref } from 'vue'
import { api, type ReadTracking } from '@/lib/api'

// One fetch per page life, shared by the library, the viewer and the Reading
// settings page, which calls refresh() after changing it.
const tracking = ref<ReadTracking | null>(null)
let loaded: Promise<void> | null = null

async function refresh() {
  try {
    tracking.value = await api.getReadTracking()
  } catch {
    // Off until the server answers; the offline banner covers the rest.
  }
}

export function useReadTracking() {
  if (!loaded) loaded = refresh()
  const enabled = computed(() => tracking.value?.enabled ?? false)
  return { tracking, enabled, refresh }
}
