// Mirrors parseSearch in internal/store/searchquery.go, only as far as the list
// needs it: which words to highlight in which column. The server is the one
// that filters, so anything this gets wrong only costs a highlight.

export const SEARCH_OPERATORS = [
  'from', 'to', 'cc', 'subject', 'filename',
  'has', 'is', 'after', 'before', 'on', 'larger', 'smaller',
] as const
export type SearchField = (typeof SEARCH_OPERATORS)[number]

export type SearchClause = {
  field: SearchField | ''
  value: string
  phrase: boolean
  negate: boolean
}

const OPERATORS = new Set<string>(SEARCH_OPERATORS)

export function parseSearch(raw: string): SearchClause[] {
  const out: SearchClause[] = []
  const rs = [...raw]
  const isSpace = (c: string | undefined) => c !== undefined && /\s/u.test(c)
  let i = 0
  while (i < rs.length) {
    while (i < rs.length && isSpace(rs[i])) i++
    if (i >= rs.length) break
    const c: SearchClause = { field: '', value: '', phrase: false, negate: false }
    if (rs[i] === '-' && i + 1 < rs.length && !isSpace(rs[i + 1])) {
      c.negate = true
      i++
    }
    let j = i
    while (j < rs.length && /\p{L}/u.test(rs[j])) j++
    const key = rs.slice(i, j).join('').toLowerCase()
    if (j > i && rs[j] === ':' && OPERATORS.has(key)) {
      c.field = key as SearchField
      i = j + 1
    }
    if (rs[i] === '"') {
      let end = i + 1
      while (end < rs.length && rs[end] !== '"') end++
      c.value = rs.slice(i + 1, end).join('')
      c.phrase = true
      i = end + 1
    } else {
      let end = i
      while (end < rs.length && !isSpace(rs[end])) end++
      c.value = rs.slice(i, end).join('')
      i = end
    }
    c.value = c.value.trim()
    if (c.value) out.push(c)
  }
  return out
}

/**
 * The words to highlight in one column: the plain search words plus any
 * operator aimed at that column. Excluded words and the other operators are
 * left out, since they never appear as matched text.
 */
export function highlightTerms(raw: string, column: 'subject' | 'from'): string {
  return parseSearch(raw)
    .filter((c) => !c.negate && (c.field === '' || c.field === column))
    .map((c) => c.value)
    .join(' ')
}
