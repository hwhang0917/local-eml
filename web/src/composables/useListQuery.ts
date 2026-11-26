import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useStorage } from '@vueuse/core'

// 500 is the server's ceiling; anything larger is silently clamped back to 50.
export const PAGE_SIZES = [10, 25, 50, 100, 200, 500] as const
export type PageSize = (typeof PAGE_SIZES)[number]
export const DEFAULT_PAGE_SIZE: PageSize = 50

/** A change to the query string; undefined or '' removes the key. */
export type QueryPatch = Record<string, string | undefined>

function normalizePageSize(n: number): PageSize {
  return (PAGE_SIZES as readonly number[]).includes(n) ? (n as PageSize) : DEFAULT_PAGE_SIZE
}

/**
 * The state of a paged list page that lives in the URL, so a filtered list
 * survives a trip to a message and back and can be bookmarked. The chosen page
 * size is also remembered under pageSizeKey for the next visit.
 */
export function useListQuery(pageSizeKey: string) {
  const route = useRoute()
  const router = useRouter()
  const { t } = useI18n()
  const storedPageSize = useStorage<PageSize>(pageSizeKey, DEFAULT_PAGE_SIZE)

  /** One query value; repeated or missing keys read as def. */
  function param(key: string, def = ''): string {
    const v = route.query[key]
    return typeof v === 'string' ? v : def
  }

  function pushQuery(patch: QueryPatch) {
    const next: Record<string, string> = {}
    for (const [k, v] of Object.entries(route.query)) {
      if (typeof v === 'string') next[k] = v
    }
    for (const [k, v] of Object.entries(patch)) {
      if (v === undefined || v === '') delete next[k]
      else next[k] = v
    }
    router.push({ query: next })
  }

  const offset = computed(() => Number(param('offset', '0')) || 0)
  const limit = computed<PageSize>(() => {
    const fromQuery = Number(param('limit'))
    return normalizePageSize(fromQuery > 0 ? fromQuery : storedPageSize.value)
  })

  function setOffset(value: number) {
    pushQuery({ offset: value > 0 ? String(value) : undefined })
  }
  function goPrevPage() {
    setOffset(Math.max(0, offset.value - limit.value))
  }
  function goNextPage() {
    setOffset(offset.value + limit.value)
  }

  function setPageSize(v: unknown) {
    if (v === null || v === undefined || v === '') return
    const size = normalizePageSize(Number(v))
    storedPageSize.value = size
    pushQuery({ limit: size === DEFAULT_PAGE_SIZE ? undefined : String(size), offset: undefined })
  }

  function setDateRange(range: { from: string; to: string }) {
    pushQuery({ from: range.from || undefined, to: range.to || undefined, offset: undefined })
  }

  /** "1–50 of 312" for a page currently showing `shown` rows. */
  function pageLabel(shown: number, total: number): string {
    if (total === 0) return t('library.page_count_zero')
    const end = Math.min(offset.value + shown, total)
    return t('library.page_count', { start: offset.value + 1, end, total })
  }

  return { param, pushQuery, offset, limit, setOffset, goPrevPage, goNextPage, setPageSize, setDateRange, pageLabel }
}
