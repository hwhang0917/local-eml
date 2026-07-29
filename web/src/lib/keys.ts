/**
 * True when a keydown should be left alone: the user is typing in a field or
 * mid-IME-composition (essential for Korean input, where every keystroke
 * composes). Escape is the caller's business — it is often meaningful even
 * inside an input.
 */
export function isTypingTarget(e: KeyboardEvent): boolean {
  if (e.isComposing) return true
  const el = e.target as HTMLElement | null
  if (!el) return false
  return (
    el.tagName === 'INPUT' ||
    el.tagName === 'TEXTAREA' ||
    el.tagName === 'SELECT' ||
    el.isContentEditable
  )
}

/** True when a modifier is held, so plain-letter shortcuts don't shadow browser ones. */
export function hasModifier(e: KeyboardEvent): boolean {
  return e.ctrlKey || e.metaKey || e.altKey
}
