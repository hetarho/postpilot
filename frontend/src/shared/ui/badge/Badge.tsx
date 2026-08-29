import { forwardRef, type HTMLAttributes } from 'react'
import { twMerge } from 'tailwind-merge'

/** Neutral by default; a STATUS chip takes the tone that matches its meaning. Colour is never the
 *  only signal — the chip always carries its text label too (design-language §2.6, §7). */
export type BadgeTone = 'neutral' | 'danger' | 'success' | 'warning' | 'info'

const TONE_STYLES: Record<BadgeTone, string> = {
  neutral: 'bg-badge-neutral-bg text-badge-neutral-fg',
  danger: 'bg-notice-danger-bg text-notice-danger-fg',
  success: 'bg-notice-success-bg text-notice-success-fg',
  warning: 'bg-notice-warning-bg text-notice-warning-fg',
  info: 'bg-notice-info-bg text-notice-info-fg',
}

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(function Badge(
  { className, tone = 'neutral', ...props },
  ref,
) {
  return (
    <span
      ref={ref}
      className={twMerge(
        // `shrink-0 whitespace-nowrap`: a badge in a flex row must never be the thing that gives
        // way, and a two-syllable Korean label must never break across two lines. `px-2 py-0.5` is
        // the §4.2 ratio for a box this small.
        'inline-flex shrink-0 items-center rounded-sm px-2 py-0.5 text-xs font-medium whitespace-nowrap',
        TONE_STYLES[tone],
        className,
      )}
      {...props}
    />
  )
})
