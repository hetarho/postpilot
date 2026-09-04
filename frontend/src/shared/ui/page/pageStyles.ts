import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** How wide a screen's content column is allowed to grow (design-language §4.5).
 *
 *  The name says what the screen IS, not how many pixels it gets — a page picks its kind once and
 *  the desk width follows from it, so widening the shell later is one edit here rather than
 *  fifteen `max-w-*` in fifteen files. */
export type PageWidth =
  /** Authored or read prose — an editor, a plan, a form. Bounded by the eye, not by the window:
   *  a line over ~75 characters loses the reader on the way back to the left margin. */
  | 'prose'
  /** A directory of rows — posts, voices, purposes, guidelines. The extra desk width goes into
   *  the row's own metadata columns, not into longer lines of text. */
  | 'wide'
  /** A data screen that is genuinely two-dimensional — leaderboards, experiment results, a
   *  side-by-side comparison. The only kind that earns the whole desk. */
  | 'board'

/** Unprefixed is the phone, and it carries NO cap — 360px is narrower than the smallest of these,
 *  so a cap there is a number that can only ever be wrong later (§1.5: delete every prefixed class
 *  and what is left must still be a finished screen). `sm:` is the laptop column the app already
 *  had, and `lg:` is the desk, where the sidebar has taken the chrome out of the content's way and
 *  the column can finally use the room. */
const WIDTH_STYLES: Record<PageWidth, string> = {
  prose: 'sm:max-w-2xl lg:max-w-3xl',
  wide: 'sm:max-w-2xl lg:max-w-5xl',
  board: 'sm:max-w-4xl lg:max-w-7xl',
}

/** The one place a screen's outer column exists. Every `main` in `pages/` wears this instead of
 *  hand-rolling `mx-auto w-full max-w-2xl px-4 py-8 sm:px-6` — the shape that had drifted into six
 *  different widths and three different paddings across fifteen screens, and that left a 1440px
 *  desk with 380px of empty gutter on either side of a phone-width column.
 *
 *  Styles, not a component: `main`, `section` and `div` all need this box and each has to keep its
 *  own element and its own `aria-labelledby` — the same reason `buttonStyles` exists beside
 *  `Button`. Conflicts resolve toward `className` (twMerge), so a page that needs `flex-1
 *  flex-col` for a docked ActionBar, or a tighter `py-6`, says so at the call site. */
export function pageStyles({
  width = 'prose',
  gutters = true,
  className,
}: {
  width?: PageWidth
  /** `false` for a screen whose list rows run edge to edge: the gutter then lives on each block
   *  instead, so a pressed row reaches the column's edge rather than stopping 16px short and
   *  reading as a card (§4.2). */
  gutters?: boolean
  className?: string
} = {}) {
  return twMerge(
    clsx(
      'mx-auto w-full py-6 sm:py-8',
      WIDTH_STYLES[width],
      gutters && 'px-4 sm:px-6 lg:px-8',
      className,
    ),
  )
}
