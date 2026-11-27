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

export type ButtonVariant = 'default' | 'secondary' | 'outline' | 'ghost' | 'destructive'
export type ButtonSize = 'sm' | 'md' | 'lg' | 'icon'

const buttonVariants: Record<ButtonVariant, string> = {
  default: 'bg-primary text-primary-foreground rounded-full hover:bg-primary/95',
  secondary: 'bg-transparent text-primary border border-primary rounded-full hover:bg-primary/5',
  outline: 'bg-pearl text-foreground border border-hairline hover:bg-accent rounded-sm',
  ghost: 'bg-transparent text-foreground hover:bg-accent rounded-sm',
  destructive: 'bg-destructive text-destructive-foreground rounded-full hover:bg-destructive/95',
}

const buttonSizes: Record<ButtonSize, string> = {
  sm: 'h-8 px-4 text-sm',
  md: 'h-10 px-5 text-sm',
  lg: 'h-11 px-6 text-base',
  icon: 'h-10 w-10 rounded-sm',
}

/** The <Button> look, for a RouterLink or <a> that should read as one. */
export function buttonClass(variant: ButtonVariant = 'default', size: ButtonSize = 'md') {
  return cn(
    'inline-flex items-center justify-center gap-2 whitespace-nowrap font-normal'
      + ' focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
      + ' focus-visible:ring-offset-background disabled:opacity-50 disabled:pointer-events-none select-none',
    buttonVariants[variant],
    buttonSizes[size],
  )
}
