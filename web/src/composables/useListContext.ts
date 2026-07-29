import { useStorage } from '@vueuse/core'
import { api, type Email } from '@/lib/api'

type ListParams = NonNullable<Parameters<typeof api.listEmails>[0]>

export interface ListCtx {
  /** The library's filter/sort params, without limit/offset. */
  params: ListParams
  /** Absolute index of the opened email in the filtered list. */
  index: number
  total: number
}

export interface Neighbors {
  prev: Email | null
  next: Email | null
  index: number
  total: number
}

// sessionStorage, not module scope: the viewer must still know its list after a
// refresh, but a new tab should not inherit another tab's browsing position.
const ctx = useStorage<ListCtx | null>('library-list-ctx', null, sessionStorage, {
  serializer: {
    read: (v) => (v ? (JSON.parse(v) as ListCtx) : null),
    write: JSON.stringify,
  },
})

async function windowAround(params: ListParams, index: number) {
  const offset = Math.max(0, index - 1)
  const r = await api.listEmails({ ...params, limit: 3, offset })
  return { r, offset }
}

/**
 * The viewer's prev/next, via one 3-row window of the same query the library
 * ran. Returns null when there is no context (direct link) or the current
 * message is no longer where the context says it is and can't be found in the
 * window — nav simply hides then.
 */
async function neighbors(sha: string): Promise<Neighbors | null> {
  const c = ctx.value
  if (!c) return null
  let { r, offset } = await windowAround(c.params, c.index)
  let pos = r.items.findIndex((e) => e.sha256 === sha)
  if (pos === -1) return null
  let abs = offset + pos
  // Self-heal a drifted index (rows imported/deleted since the list was open),
  // refetching once if the correction pushed a neighbor outside the window.
  if (abs !== c.index &&
    ((pos === 0 && abs > 0) || (pos === r.items.length - 1 && abs < r.total - 1))) {
    ;({ r, offset } = await windowAround(c.params, abs))
    pos = r.items.findIndex((e) => e.sha256 === sha)
    if (pos === -1) return null
    abs = offset + pos
  }
  ctx.value = { ...c, index: abs, total: r.total }
  return {
    prev: pos > 0 ? r.items[pos - 1] : null,
    next: pos < r.items.length - 1 ? r.items[pos + 1] : null,
    index: abs,
    total: r.total,
  }
}

export function useListContext() {
  return {
    ctx,
    set(value: ListCtx) {
      ctx.value = value
    },
    shift(delta: number) {
      if (ctx.value) ctx.value = { ...ctx.value, index: ctx.value.index + delta }
    },
    neighbors,
  }
}
