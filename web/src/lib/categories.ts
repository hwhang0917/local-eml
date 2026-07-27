// The palette is the whole set of categories: one row per colour, seeded by the
// server and thereafter only renamed, the way Finder's colour tags work.
// Mirrors CategoryColors in internal/store/categories.go — that list is the
// authority; keep these two in step.
export const CATEGORY_COLORS = ['red', 'orange', 'yellow', 'green', 'blue', 'purple', 'grey'] as const

export type CategoryColor = (typeof CATEGORY_COLORS)[number]

// Tailwind scans source for literal class names, so `bg-${color}-500` compiles
// to nothing. This map is the only place a token becomes a class.
//
// The 500 level reads against both the light (#ffffff) and dark (#1d1d1f)
// backgrounds, so no dark: variants are needed. Yellow goes a step lighter for
// contrast on white, and grey uses zinc to sit between the two backgrounds.
const DOT_CLASS: Record<CategoryColor, string> = {
  red: 'bg-red-500',
  orange: 'bg-orange-500',
  yellow: 'bg-yellow-400',
  green: 'bg-green-500',
  blue: 'bg-blue-500',
  purple: 'bg-purple-500',
  grey: 'bg-zinc-400',
}

export function dotClass(color: string | undefined): string {
  return DOT_CLASS[color as CategoryColor] ?? DOT_CLASS.grey
}
