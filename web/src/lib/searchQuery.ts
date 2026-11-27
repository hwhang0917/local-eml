// Mirrors parseSearch in internal/store/searchquery.go, only as far as the list
// needs it: which words to highlight in which column. The server is the one
// that filters, so anything this gets wrong only costs a highlight.

export const SEARCH_OPERATORS = [
  'from', 'to', 'cc', 'subject', 'filename',
  'has', 'is', 'after', 'before', 'on', 'until', 'larger', 'smaller', 'category',
] as const
export type SearchField = (typeof SEARCH_OPERATORS)[number]

export type SearchClause = {
  field: SearchField | ''
  value: string
  phrase: boolean
  negate: boolean
  /** Where the clause sits in the raw query, as [start, end) code point offsets. */
  start: number
  end: number
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
    const c: SearchClause = { field: '', value: '', phrase: false, negate: false, start: i, end: i }
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
    c.end = Math.min(i, rs.length)
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

// The filter buttons beside the search box don't keep state of their own: each
// one reads and writes a clause of the query, so what the box says is always
// the whole filter, and a saved search is just its text.

const sameField = (c: SearchClause, field: SearchField) => c.field === field && !c.negate

/** Whether the query has field:value, not negated (case-insensitive). */
export function hasClause(raw: string, field: SearchField, value: string): boolean {
  const v = value.toLowerCase()
  return parseSearch(raw).some((c) => sameField(c, field) && c.value.toLowerCase() === v)
}

/** The value of the first non-negated clause for field, or ''. */
export function clauseValue(raw: string, field: SearchField): string {
  return parseSearch(raw).find((c) => sameField(c, field))?.value ?? ''
}

/** Quotes a value that has spaces, so it stays one clause. */
export function clauseToken(field: SearchField, value: string): string {
  return `${field}:${/\s/u.test(value) ? `"${value.replace(/"/g, '')}"` : value}`
}

/**
 * Removes the non-negated clauses that match, then appends tokens. Everything
 * else the person typed stays where it was.
 */
export function replaceClauses(
  raw: string,
  match: (c: SearchClause) => boolean,
  tokens: string[] = [],
): string {
  const rs = [...raw]
  const drop = parseSearch(raw).filter((c) => !c.negate && match(c))
  for (const c of [...drop].reverse()) rs.splice(c.start, c.end - c.start)
  const kept = rs.join('').replace(/\s+/gu, ' ').trim()
  return [kept, ...tokens].filter(Boolean).join(' ')
}

/** Adds field:value, or takes it out when it's already there. */
export function toggleClause(raw: string, field: SearchField, value: string): string {
  const v = value.toLowerCase()
  if (hasClause(raw, field, value)) {
    return replaceClauses(raw, (c) => c.field === field && c.value.toLowerCase() === v)
  }
  return replaceClauses(raw, () => false, [clauseToken(field, value)])
}
