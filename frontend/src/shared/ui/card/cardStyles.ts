import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** Guide: the raised card — the one lifted plane a step screen, a list item or a settings group
 *  sits on. 24px corners, a soft wide shadow, generous padding. Styles, not a component, so a
 *  `section`, an `li` or a router link can each keep its own element (as `buttonStyles` does).
 *
 *  `interactive` is for a card that IS a link or a button: it lifts a touch on hover and sinks on
 *  press, so a whole-card target reads as one. */
export function cardStyles({
  padding = 'md',
  interactive = false,
  className,
}: {
  padding?: 'none' | 'sm' | 'md' | 'lg'
  interactive?: boolean
  className?: string
} = {}) {
  return twMerge(
    clsx(
      // `ui-card`: the hook a secondary button standing on the card reads its plane from.
      'ui-card bg-card-bg rounded-xl shadow-md',
      padding === 'sm' && 'p-4',
      padding === 'md' && 'p-5 sm:p-6',
      padding === 'lg' && 'p-6 sm:p-8',
      interactive &&
        'duration-base ease-standard hover:bg-card-bg-hover active:bg-card-bg-active transition-[background-color,transform,box-shadow] hover:-translate-y-0.5 hover:shadow-lg active:translate-y-0 active:scale-[0.99]',
      className,
    ),
  )
}

/** Guide: the section chip — the pill a channel's sections (글 · 템플릿 · 말투 · 지침) scroll as,
 *  under the large page title. The current chip is inverted ink-on-paper. */
export function chipStyles({
  current = false,
  className,
}: { current?: boolean; className?: string } = {}) {
  return twMerge(
    clsx(
      'duration-fast ease-standard inline-flex min-h-10 shrink-0 items-center justify-center gap-1.5 rounded-full px-4 text-[0.9375rem] font-semibold whitespace-nowrap transition-colors pointer-coarse:min-h-11',
      current
        ? 'bg-chip-bg-current text-chip-fg-current'
        : 'bg-chip-bg text-chip-fg hover:bg-chip-bg-hover active:bg-card-bg-active shadow-sm',
      className,
    ),
  )
}
