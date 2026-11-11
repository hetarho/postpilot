import { clsx } from 'clsx'
import { cardStyles } from '@/shared/ui'

/** Guide: a list card with links of its own under the title answers the pointer as ONE plane,
 *  the way 전체's whole-card links do. Its main link is stretched over the card
 *  (`workCardLinkClass`), so a hover or a press anywhere on it — the title, the gap, the row of
 *  links under it — tints and lifts the whole card; the link used to tint only the half it
 *  covered. The row's own links sit above the stretch (`workCardActionClass`) and stay their own
 *  targets. The latest unfinished card keeps its accent tint with the same density as its peers. */
export function workCardStyles({ latest = false }: { latest?: boolean } = {}) {
  return cardStyles({
    padding: 'none',
    interactive: true,
    className: clsx(
      'relative flex flex-col overflow-hidden',
      latest && 'bg-badge-accent-bg hover:bg-badge-accent-bg active:bg-badge-accent-bg',
    ),
  })
}

/** The card's main link: its content row, and the stretched hit area the whole card answers by. */
export const workCardLinkClass =
  'flex min-h-11 min-w-0 items-start gap-4 p-4 after:absolute after:inset-0 sm:p-5'

/** A link in the card's lower row, lifted above the main link's stretched area. */
export const workCardActionClass = 'relative z-10'

/** Recovery/export targets share one compact row across the card's useful phone width. */
export const workCardFooterClass =
  'flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 px-4 pb-3 sm:px-5'
