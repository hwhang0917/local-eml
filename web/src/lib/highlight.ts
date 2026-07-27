// Mirrors internal/store/chosung.go so the list highlights exactly what the
// server matched. Two modes, chosen the same way the query is: a query holding
// compatibility jamo is a 초성검색 and folds Hangul syllables to their initial
// consonant; anything else is plain text and only case-folds, because folding
// syllables there would highlight 하고 for the query 한국.
const CHOSUNG = [
  'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ',
  'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ',
]

const HANGUL_BASE = 0xac00
const HANGUL_LAST = 0xd7a3
const JAMO_PER_CHOSUNG = 21 * 28

export function hasJamo(s: string): boolean {
  for (const c of s) {
    const cp = c.codePointAt(0)!
    if (cp >= 0x3131 && cp <= 0x318e) return true
  }
  return false
}

// Folds one character to exactly one character. Length has to be preserved so
// match offsets in the folded form still index the original characters —
// toLowerCase can expand (İ becomes two chars), so those are left alone.
function foldChar(c: string, chosung: boolean): string {
  const cp = c.codePointAt(0)!
  if (chosung && cp >= HANGUL_BASE && cp <= HANGUL_LAST) {
    return CHOSUNG[Math.floor((cp - HANGUL_BASE) / JAMO_PER_CHOSUNG)]
  }
  const lower = c.toLowerCase()
  return [...lower].length === 1 ? lower : c
}

export type Segment = { text: string; match: boolean }

// Matching runs over arrays of characters rather than joined strings: an
// astral character (emoji) is two UTF-16 units, so string indices would drift
// out of step with the original text.
function findRanges(haystack: string[], needle: string[]): Array<[number, number]> {
  const found: Array<[number, number]> = []
  if (needle.length === 0 || needle.length > haystack.length) return found
  outer: for (let i = 0; i <= haystack.length - needle.length; i++) {
    for (let j = 0; j < needle.length; j++) {
      if (haystack[i + j] !== needle[j]) continue outer
    }
    found.push([i, i + needle.length])
  }
  return found
}

function mergeRanges(ranges: Array<[number, number]>): Array<[number, number]> {
  if (ranges.length === 0) return ranges
  const sorted = [...ranges].sort((a, b) => a[0] - b[0])
  const merged: Array<[number, number]> = [sorted[0]]
  for (const [start, end] of sorted.slice(1)) {
    const last = merged[merged.length - 1]
    if (start <= last[1]) last[1] = Math.max(last[1], end)
    else merged.push([start, end])
  }
  return merged
}

/**
 * Splits text into alternating plain and matching segments for the query.
 * Every term must be highlighted independently — the server ANDs them, so a
 * row can match on terms that appear in any order.
 */
export function highlightSegments(text: string, query: string): Segment[] {
  const trimmed = query.trim()
  if (!text || !trimmed) return text ? [{ text, match: false }] : []

  const chosung = hasJamo(trimmed)
  const chars = [...text]
  const folded = chars.map((c) => foldChar(c, chosung))

  const ranges: Array<[number, number]> = []
  for (const term of trimmed.split(/\s+/)) {
    const foldedTerm = [...term].map((c) => foldChar(c, chosung))
    ranges.push(...findRanges(folded, foldedTerm))
  }

  const merged = mergeRanges(ranges)
  if (merged.length === 0) return [{ text, match: false }]

  const segments: Segment[] = []
  let cursor = 0
  for (const [start, end] of merged) {
    if (start > cursor) segments.push({ text: chars.slice(cursor, start).join(''), match: false })
    segments.push({ text: chars.slice(start, end).join(''), match: true })
    cursor = end
  }
  if (cursor < chars.length) segments.push({ text: chars.slice(cursor).join(''), match: false })
  return segments
}
