import { computed } from 'vue'
import { breakpointsTailwind, useWindowSize } from '@vueuse/core'
import { textScale } from '@/lib/textSize'

// The window widths the tables drop columns at, on Tailwind's breakpoints so
// they line up with the md:/lg:/xl: classes around them at the original text
// size (S). The app runs from 640px up (a window snapped to half a scaled
// laptop screen); narrower than that App.vue shows the "widen the window"
// notice instead.
//
// Columns are sized in rem, so with larger text (Settings > Appearance) they
// need proportionally more room: each width scales with the text. At XL a
// column that comes back at 768px under S comes back at 960px.
const { width } = useWindowSize({ includeScrollbar: true })
const atLeast = (px: number) => computed(() => width.value >= px * textScale.value)
export const atLeastMd = atLeast(breakpointsTailwind.md)
export const atLeastLg = atLeast(breakpointsTailwind.lg)
export const atLeastXl = atLeast(breakpointsTailwind.xl)
