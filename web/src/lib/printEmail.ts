// Printing a message. The viewer shows the HTML body in a fixed-height iframe,
// so the browser's own print came out as one clipped page with a scrollbar:
// an iframe never breaks across pages. Here the whole message becomes the
// top-level document of a hidden frame (headers, attachment list, then the
// body as ordinary flowing content) and that frame is printed instead.

export interface PrintField {
  label: string
  value: string
}

export interface PrintAttachment {
  name: string
  detail: string
}

export interface PrintContent {
  lang: string
  subject: string
  fields: PrintField[]
  attachmentsLabel: string
  attachments: PrintAttachment[]
  /** Sanitized HTML from the server's html endpoint, or null for a text body. */
  html: string | null
  text: string
}

function esc(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// The same lockdown the server sends with the HTML body (csp() in
// handlers_emails.go): no scripts, no frames, images only from here unless the
// reader chose to load remote ones.
function csp(showRemote: boolean): string {
  return [
    "default-src 'none'",
    `img-src 'self' data:${showRemote ? ' http: https:' : ''}`,
    "style-src 'unsafe-inline'",
    "font-src 'self' data:",
  ].join('; ')
}

const STYLE = `
  @page { margin: 15mm; }
  html, body { margin: 0; padding: 0; background: #fff; color: #111; }
  body { font: 13px/1.5 Pretendard, system-ui, -apple-system, "Segoe UI", "Malgun Gothic", "Apple SD Gothic Neo", sans-serif; }
  .print-head { border-bottom: 1px solid #bbb; padding-bottom: 10px; margin-bottom: 16px; break-inside: avoid; }
  .print-head h1 { font-size: 18px; line-height: 1.3; margin: 0 0 8px; overflow-wrap: anywhere; }
  .print-head dl { display: grid; grid-template-columns: max-content 1fr; gap: 2px 14px; margin: 0; }
  .print-head dt { color: #555; }
  .print-head dd { margin: 0; overflow-wrap: anywhere; }
  .print-head ul { margin: 0; padding-left: 18px; }
  .print-head .detail { color: #555; }
  .print-body { overflow-wrap: anywhere; }
  .print-body img { max-width: 100%; height: auto; }
  .print-body table { max-width: 100%; }
  pre.print-body { white-space: pre-wrap; font: 12px/1.45 ui-monospace, Menlo, Consolas, "D2Coding", monospace; margin: 0; }
  .print-toc h1 { font-size: 20px; margin: 0 0 4px; }
  .print-toc .detail, .print-index { color: #555; }
  .print-toc table { width: 100%; border-collapse: collapse; margin-top: 12px; font-size: 12px; }
  .print-toc th, .print-toc td { text-align: left; padding: 3px 6px; border-bottom: 1px solid #ddd; vertical-align: top; overflow-wrap: anywhere; }
  .print-toc .num { width: 2.5em; color: #555; }
  .print-msg { break-before: page; }
  .print-index { font-size: 11px; margin: 0 0 6px; }
`

// One message: its header block, then its body as ordinary flowing content.
function printSection(c: PrintContent): string {
  const rows = c.fields
    .filter((f) => f.value)
    .map((f) => `<dt>${esc(f.label)}</dt><dd>${esc(f.value)}</dd>`)
  if (c.attachments.length) {
    const items = c.attachments
      .map((a) => `<li>${esc(a.name)} <span class="detail">(${esc(a.detail)})</span></li>`)
      .join('')
    rows.push(`<dt>${esc(c.attachmentsLabel)}</dt><dd><ul>${items}</ul></dd>`)
  }
  // The HTML body has already been through the server's sanitizer (no
  // scripts, handlers or <style>), and the frame runs no scripts anyway.
  const body = c.html != null
    ? `<div class="print-body">${c.html}</div>`
    : `<pre class="print-body">${esc(c.text)}</pre>`
  return `<header class="print-head">
<h1>${esc(c.subject)}</h1>
<dl>${rows.join('')}</dl>
</header>
${body}`
}

function wrapDocument(lang: string, title: string, body: string, showRemote: boolean): string {
  return `<!doctype html>
<html lang="${esc(lang)}">
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="${esc(csp(showRemote))}">
<title>${esc(title)}</title>
<style>${STYLE}</style>
</head>
<body>
${body}
</body>
</html>`
}

export function buildPrintDocument(c: PrintContent, showRemote: boolean): string {
  return wrapDocument(c.lang, c.subject, printSection(c), showRemote)
}

export interface PrintBundle {
  lang: string
  title: string
  /** Shown above the contents table, e.g. "12 messages · printed 3 May 2025". */
  summary: string
  columns: { date: string; from: string; subject: string }
  messages: (PrintContent & { date: string; from: string })[]
}

/**
 * Several messages in one printout: a contents page listing them, then each
 * message starting on a new page. Saved as PDF, it's one file to hand over.
 */
export function buildPrintBundle(b: PrintBundle, showRemote: boolean): string {
  const rows = b.messages
    .map((m, i) => `<tr><td class="num">${i + 1}</td><td>${esc(m.date)}</td><td>${esc(m.from)}</td><td>${esc(m.subject)}</td></tr>`)
    .join('')
  const toc = `<section class="print-toc">
<h1>${esc(b.title)}</h1>
<p class="detail">${esc(b.summary)}</p>
<table><thead><tr><th class="num">#</th><th>${esc(b.columns.date)}</th><th>${esc(b.columns.from)}</th><th>${esc(b.columns.subject)}</th></tr></thead>
<tbody>${rows}</tbody></table>
</section>`
  const sections = b.messages
    .map((m, i) => `<article class="print-msg"><p class="print-index">${i + 1} / ${b.messages.length}</p>${printSection(m)}</article>`)
    .join('\n')
  return wrapDocument(b.lang, b.title, toc + sections, showRemote)
}

let current: HTMLIFrameElement | null = null

function cleanup() {
  current?.remove()
  current = null
}

/** Waits for the frame's images so the printout doesn't have empty boxes. */
async function imagesReady(doc: Document) {
  const pending = Array.from(doc.images)
    .filter((img) => !img.complete)
    .map((img) => new Promise<void>((resolve) => {
      img.addEventListener('load', () => resolve(), { once: true })
      img.addEventListener('error', () => resolve(), { once: true })
    }))
  // A stalled remote image must not hold the print dialog back for ever.
  await Promise.race([Promise.all(pending), new Promise((r) => setTimeout(r, 5000))])
}

/** Opens the browser's print dialog for a document built by buildPrintDocument. */
export function printDocument(html: string): Promise<void> {
  cleanup()
  const frame = document.createElement('iframe')
  // allow-same-origin: the parent reaches in to call print() and to load
  // cid: images from our API. allow-modals: print() is a modal and is
  // refused in a sandbox without it. No allow-scripts, as in the viewer.
  frame.setAttribute('sandbox', 'allow-same-origin allow-modals')
  frame.setAttribute('aria-hidden', 'true')
  frame.setAttribute('tabindex', '-1')
  frame.dataset.testid = 'print-frame'
  // Off-screen but laid out: a display:none frame prints blank in some browsers.
  frame.style.cssText = 'position:fixed;left:-10000px;top:0;width:800px;height:600px;border:0;'
  current = frame
  return new Promise<void>((resolve, reject) => {
    frame.addEventListener('load', async () => {
      const win = frame.contentWindow
      if (!win) {
        cleanup()
        reject(new Error('print frame did not load'))
        return
      }
      await imagesReady(win.document)
      win.addEventListener('afterprint', () => setTimeout(cleanup, 0), { once: true })
      win.focus()
      win.print()
      resolve()
    }, { once: true })
    frame.srcdoc = html
    document.body.appendChild(frame)
  })
}
