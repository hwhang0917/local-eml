import { watch } from 'vue'
import { api, type MailEvent } from '@/lib/api'
import { locked } from '@/lib/auth'

// One live stream from the server saying "new mail came in", shared by every
// open tab. Browsers allow only six connections per site, so a stream per tab
// would starve the rest of the app once a few tabs are open. Instead the tabs
// elect one leader with a Web Lock: it holds the EventSource and relays each
// event to the others over a BroadcastChannel. When the leader tab closes,
// the lock passes to the next tab, which opens the stream there.

export type MailSignal =
  /** A job added mail to the library. */
  | { type: 'new-mail'; added: number; kind: string }
  /**
   * The stream came back after a gap. Anything that arrived meanwhile was not
   * announced, so listeners should catch up.
   */
  | { type: 'resumed' }

type Listener = (sig: MailSignal) => void

const LOCK_NAME = 'local-eml-mail-events'
const CHANNEL_NAME = 'local-eml-mail-events'
const RETRY_MIN_MS = 2_000
const RETRY_MAX_MS = 60_000

const listeners = new Set<Listener>()
let started = false
let channel: BroadcastChannel | null = null

function emit(sig: MailSignal) {
  for (const l of listeners) {
    try {
      l(sig)
    } catch {
      /* one listener failing must not stop the others */
    }
  }
}

/** Delivers sig in this tab and every other open tab. */
function fanOut(sig: MailSignal) {
  emit(sig)
  channel?.postMessage(sig)
}

/**
 * Keeps an EventSource open for as long as this tab leads. The browser
 * reconnects a dropped stream by itself; one it gave up on (the server was
 * down, or the session is locked) is reopened here with backoff.
 */
function lead() {
  let es: EventSource | null = null
  let retryMs = RETRY_MIN_MS
  let retryTimer: number | null = null
  // Every "ready" after the first means the stream was away for a while.
  let connectedBefore = false

  const open = () => {
    retryTimer = null
    if (locked.value) return // reopened by the watch below once unlocked
    es = api.mailEventSource()
    es.onmessage = (msg) => {
      let ev: MailEvent
      try {
        ev = JSON.parse(msg.data)
      } catch {
        return
      }
      if (ev.type === 'ready') {
        retryMs = RETRY_MIN_MS
        if (connectedBefore) fanOut({ type: 'resumed' })
        connectedBefore = true
      } else if (ev.type === 'new-mail') {
        fanOut({ type: 'new-mail', added: ev.added ?? 0, kind: ev.kind ?? '' })
      }
    }
    es.onerror = () => {
      if (es?.readyState !== EventSource.CLOSED) return // the browser is retrying
      es = null
      retryTimer = window.setTimeout(open, retryMs)
      retryMs = Math.min(retryMs * 2, RETRY_MAX_MS)
    }
  }

  watch(locked, (isLocked) => {
    if (isLocked) {
      es?.close()
      es = null
    } else if (!es && retryTimer == null) {
      open()
    }
  })
  open()
}

/** Starts listening once for the page. Safe to call more than once. */
export function startMailEvents() {
  if (started) return
  started = true
  if (typeof BroadcastChannel !== 'undefined') {
    channel = new BroadcastChannel(CHANNEL_NAME)
    channel.onmessage = (msg: MessageEvent<MailSignal>) => emit(msg.data)
  }
  if (channel && typeof navigator !== 'undefined' && navigator.locks) {
    // The callback's promise never settles, so the lock is held until the
    // tab goes away.
    void navigator.locks.request(LOCK_NAME, () => {
      lead()
      return new Promise<never>(() => {})
    })
  } else {
    lead()
  }
}

/** Calls fn for every mail signal until the returned function is called. */
export function onMailSignal(fn: Listener): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}
