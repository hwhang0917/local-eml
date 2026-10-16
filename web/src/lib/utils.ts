import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/** Plain text/number/file input in a form card. */
export const fieldClass =
  'w-full rounded-md border border-border bg-background px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-primary'

/** One button of a segmented switch (provider/mode tabs). */
export function segmentClass(active: boolean) {
  return cn(
    'px-4 py-1.5 text-sm rounded-md cursor-pointer transition-colors',
    active ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground',
  )
}

/** Native <dialog> shown via showModal(); content supplies its own padding. */
export const dialogClass =
  'm-auto w-[90vw] max-w-md rounded-lg border bg-background p-0 text-foreground backdrop:bg-black/50'
