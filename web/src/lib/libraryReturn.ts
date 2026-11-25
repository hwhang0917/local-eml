import type { RouteLocationNormalized, Router } from 'vue-router'

// "Back to library" must land on the library the user left, with its filters,
// page and scroll intact, however many emails they stepped through since
// (prev/next, thread links, attachment jumps). A single history.back() only
// works when the library is the entry right before the viewer, so we remember
// where in this tab's history the library entry sits and jump straight to it.
//
// sessionStorage, like the list context: it has to survive a refresh (history
// positions do too) but must not leak into another tab.

const KEY = 'library-return'

interface LibraryEntry {
  /** vue-router's history position of the library entry. */
  position: number
  /** Its full path, for when the entry is gone and we have to navigate. */
  fullPath: string
  /** False once a later push overwrote that history slot. */
  live: boolean
}

function read(): LibraryEntry | null {
  try {
    const v = sessionStorage.getItem(KEY)
    return v ? (JSON.parse(v) as LibraryEntry) : null
  } catch {
    return null
  }
}

function write(v: LibraryEntry) {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(v))
  } catch {
    // Storage blocked: the button still works, it just navigates fresh.
  }
}

function currentPosition(router: Router): number | null {
  const p = router.options.history.state.position
  return typeof p === 'number' ? p : null
}

export function trackLibraryEntry(router: Router) {
  router.afterEach((to: RouteLocationNormalized, _from, failure) => {
    if (failure) return
    const pos = currentPosition(router)
    if (pos === null) return
    if (to.name === 'library') {
      write({ position: pos, fullPath: to.fullPath, live: true })
      return
    }
    // A navigation that lands at or before the library's slot means that slot
    // now holds something else (or will be overwritten by the next push).
    const saved = read()
    if (saved?.live && pos <= saved.position) write({ ...saved, live: false })
  })
}

export function returnToLibrary(router: Router) {
  const saved = read()
  const pos = currentPosition(router)
  if (saved?.live && pos !== null && saved.position < pos) {
    router.go(saved.position - pos)
    return
  }
  // No library entry behind us (direct link, new tab): open it fresh, still
  // with the last filters the user had if we know them.
  router.push(saved?.fullPath ?? { name: 'library' })
}
