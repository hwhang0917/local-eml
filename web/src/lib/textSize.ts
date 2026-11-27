import { useStorage } from '@vueuse/core'
import { watchEffect } from 'vue'

/**
 * UI text size. The theme sizes text, spacing and controls in rem, so the
 * whole interface follows the root font-size. Each step is a percentage of
 * the browser's own default (usually 16px), so a larger default set in the
 * browser still counts.
 *
 * "sm" is the size the app shipped with before this setting existed; the
 * default "md" is one step up, because menus and labels read small at 16px.
 * Layout breakpoints are media queries on the window width in px, so they
 * don't move with this: at a larger size, columns drop at the same window
 * widths as before.
 */
export type TextSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl'

export const TEXT_SIZES: readonly TextSize[] = ['xs', 'sm', 'md', 'lg', 'xl']

const rootPercent: Record<TextSize, number> = {
  xs: 87.5, // 14px
  sm: 100, // 16px
  md: 106.25, // 17px
  lg: 112.5, // 18px
  xl: 125, // 20px
}

export const textSize = useStorage<TextSize>('settings-text-size', 'md')

/**
 * Applies the saved size to <html> and keeps it in sync. Called from main.ts
 * before the app mounts: the page has no text until then, so the first paint
 * is already at the right size.
 */
export function applyTextSize() {
  watchEffect(() => {
    const pct = rootPercent[textSize.value] ?? rootPercent.md
    document.documentElement.style.fontSize = `${pct}%`
  })
}
