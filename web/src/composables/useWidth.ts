import { breakpointsTailwind, useBreakpoints } from '@vueuse/core'

// The window widths the tables drop columns at, on Tailwind's breakpoints so
// they line up with the md:/lg:/xl: classes around them. The app runs from
// 640px up (a window snapped to half a scaled laptop screen); narrower than
// that App.vue shows the "widen the window" notice instead.
const bp = useBreakpoints(breakpointsTailwind)
export const atLeastMd = bp.greaterOrEqual('md')
export const atLeastLg = bp.greaterOrEqual('lg')
export const atLeastXl = bp.greaterOrEqual('xl')
