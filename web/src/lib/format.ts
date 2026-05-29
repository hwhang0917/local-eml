import { useStorage } from '@vueuse/core'
import { i18n } from '@/i18n'

export function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`
  return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

export type DateFormat = 'absolute' | 'relative'
export const dateFormat = useStorage<DateFormat>('settings-date-format', 'absolute')

export function formatDate(iso: string | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime()) || d.getFullYear() < 2) return ''
  const locale = i18n.global.locale.value
  if (dateFormat.value === 'relative') return formatRelative(d, locale)
  return d.toLocaleString(locale, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function formatRelative(d: Date, locale: string): string {
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  const diffSec = Math.round((d.getTime() - Date.now()) / 1000)
  const abs = Math.abs(diffSec)
  if (abs < 60) return rtf.format(diffSec, 'second')
  const min = Math.round(diffSec / 60)
  if (Math.abs(min) < 60) return rtf.format(min, 'minute')
  const hr = Math.round(min / 60)
  if (Math.abs(hr) < 24) return rtf.format(hr, 'hour')
  const day = Math.round(hr / 24)
  if (Math.abs(day) < 7) return rtf.format(day, 'day')
  const week = Math.round(day / 7)
  if (Math.abs(week) < 5) return rtf.format(week, 'week')
  const month = Math.round(day / 30)
  if (Math.abs(month) < 12) return rtf.format(month, 'month')
  const year = Math.round(day / 365)
  return rtf.format(year, 'year')
}

export function shortSHA(s: string): string {
  return s.slice(0, 8)
}
