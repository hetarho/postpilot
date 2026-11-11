import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'

/** One row of a `ListGroup`: a 56px target with an optional glyph chip, the row's name, and a
 *  trailing value or chevron. Styles, so a router link, a button or a plain `div` keeps its own
 *  element. */
export function listRowStyles({
  interactive = true,
  className,
}: { interactive?: boolean; className?: string } = {}) {
  return twMerge(
    clsx(
      'text-content-primary flex min-h-14 w-full items-center gap-3 px-4 py-3 text-left text-body leading-snug',
      interactive && 'hover:bg-row-bg-hover active:bg-row-bg-active transition-colors',
      className,
    ),
  )
}
