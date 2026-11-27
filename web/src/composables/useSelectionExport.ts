import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { toast } from 'vue-sonner'
import { api, type Email } from '@/lib/api'
import { attachmentName, formatBytes, formatDateAbsolute } from '@/lib/format'
import { buildPrintBundle, printDocument } from '@/lib/printEmail'

export type ExportFormat = 'zip' | 'mbox' | 'pdf'

/** What to export: the checked rows, or every message the search matches. */
export type ExportTarget =
  | { kind: 'selection'; shas: string[]; threads: boolean }
  | { kind: 'search'; q: string; total: number }

// A printout fetches and lays out every body in one page; past this many the
// browser's print preview crawls, and a zip or mbox is the better tool.
export const MAX_PRINT = 200
const PRINT_CONCURRENCY = 4

const FRAME_NAME = 'local-eml-export-download'

// The download goes through a plain form post so the browser streams the file
// to disk, however large, instead of fetch holding it in memory. It targets a
// hidden frame so an error response can't navigate the app away.
function postDownload(fields: [string, string][]) {
  let frame = document.querySelector<HTMLIFrameElement>(`iframe[name="${FRAME_NAME}"]`)
  if (!frame) {
    frame = document.createElement('iframe')
    frame.name = FRAME_NAME
    frame.setAttribute('aria-hidden', 'true')
    frame.tabIndex = -1
    frame.style.display = 'none'
    document.body.appendChild(frame)
  }
  const form = document.createElement('form')
  form.method = 'POST'
  form.action = api.exportSelectionURL()
  form.target = FRAME_NAME
  form.style.display = 'none'
  for (const [name, value] of fields) {
    const input = document.createElement('input')
    input.type = 'hidden'
    input.name = name
    input.value = value
    form.appendChild(input)
  }
  document.body.appendChild(form)
  form.submit()
  form.remove()
}

// The contents table has room for the whole name; the library's senderName
// shortens it to fit a column.
function fullSender(addr: string): string {
  const m = /^\s*"?([^"<]*?)"?\s*<([^>]*)>/.exec(addr)
  return (m && (m[1].trim() || m[2])) || addr.replace(/^<|>$/g, '')
}

async function mapLimit<T, R>(items: T[], limit: number, fn: (x: T) => Promise<R>): Promise<R[]> {
  const out: R[] = new Array(items.length)
  let next = 0
  async function worker() {
    while (next < items.length) {
      const i = next++
      out[i] = await fn(items[i])
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker))
  return out
}

export function useSelectionExport() {
  const { t, locale } = useI18n()
  const busy = ref(false)

  /** The messages a target stands for, oldest first, for printing. */
  async function resolveEmails(target: ExportTarget, rows: Email[]): Promise<Email[]> {
    if (target.kind === 'search') {
      const r = await api.listEmails({ q: target.q || undefined, sort: 'sent_at', order: 'asc', limit: MAX_PRINT, offset: 0 })
      return r.items
    }
    const picked = new Set(target.shas)
    const out = new Map<string, Email>()
    for (const e of rows) {
      if (!picked.has(e.sha256)) continue
      if (target.threads && (e.thread_count ?? 0) > 1) {
        for (const m of (await api.getThread(e.sha256)).items) out.set(m.sha256, m)
      } else {
        out.set(e.sha256, e)
      }
    }
    return [...out.values()].sort((a, b) => a.sent_at.localeCompare(b.sent_at))
  }

  async function printAll(emails: Email[]) {
    const lang = locale.value
    const messages = await mapLimit(emails, PRINT_CONCURRENCY, async (e) => {
      const p = await api.getParts(e.sha256)
      const html = p.has_html ? await api.getHTML(e.sha256, false, lang) : null
      const text = html == null && p.has_text ? await api.getText(e.sha256) : ''
      const date = formatDateAbsolute(e.sent_at)
      return {
        lang,
        subject: e.subject || t('library.no_subject'),
        fields: [
          { label: t('viewer.from'), value: e.from },
          { label: t('viewer.to'), value: e.to.join(', ') },
          { label: t('viewer.cc'), value: e.cc.join(', ') },
          { label: t('viewer.date'), value: date },
        ],
        attachmentsLabel: t('viewer.tabs.attachments'),
        attachments: p.attachments.map((a) => ({ name: attachmentName(a), detail: formatBytes(a.size) })),
        html,
        text,
        date,
        from: fullSender(e.from),
      }
    })
    const doc = buildPrintBundle({
      lang,
      title: t('export_selection.print_title'),
      summary: t('export_selection.print_summary', {
        count: emails.length,
        date: formatDateAbsolute(new Date().toISOString()),
      }, emails.length),
      columns: { date: t('library.col.date'), from: t('library.col.from'), subject: t('library.col.subject') },
      messages,
    }, false)
    await printDocument(doc)
  }

  async function run(format: ExportFormat, target: ExportTarget, rows: Email[] = []) {
    if (busy.value) return
    if (format !== 'pdf') {
      const fields: [string, string][] = [['format', format]]
      if (target.kind === 'selection') {
        for (const sha of target.shas) fields.push(['sha', sha])
        if (target.threads) fields.push(['threads', '1'])
      } else if (target.q) {
        fields.push(['q', target.q])
      }
      postDownload(fields)
      toast.success(t('export_selection.started'))
      return
    }
    if (target.kind === 'search' && target.total > MAX_PRINT) {
      toast.error(t('export_selection.too_many_to_print', { max: MAX_PRINT }))
      return
    }
    busy.value = true
    const loading = toast.loading(t('export_selection.preparing_print'))
    try {
      const emails = await resolveEmails(target, rows)
      if (emails.length > MAX_PRINT) {
        toast.error(t('export_selection.too_many_to_print', { max: MAX_PRINT }))
        return
      }
      await printAll(emails)
    } catch (err) {
      toast.error(t('viewer.print_error'), { description: String(err) })
    } finally {
      toast.dismiss(loading)
      busy.value = false
    }
  }

  return { busy, run }
}
