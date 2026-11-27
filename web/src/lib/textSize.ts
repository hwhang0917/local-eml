import { useStorage } from '@vueuse/core'
import { computed, watchEffect } from 'vue'

/**
 * UI text size. The theme sizes text, spacing and controls in rem, so the
 * whole interface follows the root font-size. Each step is a percentage of
 * the browser's own default (usually 16px), so a larger default set in the
 * browser still counts.
 *
 * "sm" is the size the app shipped with before this setting existed; the
 * default "md" is one step up, because menus and labels read small at 16px.
 * CSS breakpoints are media queries on the window width in px and don't move
 * with this. The table column drops in useWidth.ts do: bigger text needs a
 * wider window before a column comes back.
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

/** How much larger than the browser default the UI text is (1 = S). */
export const textScale = computed(() => (rootPercent[textSize.value] ?? rootPercent.md) / 100)

/**
 * Applies the saved size to <html> and keeps it in sync. Called from main.ts
 * before the app mounts: the page has no text until then, so the first paint
 * is already at the right size.
 */
export function applyTextSize() {
  watchEffect(() => {
    document.documentElement.style.fontSize = `${textScale.value * 100}%`
  })
}
