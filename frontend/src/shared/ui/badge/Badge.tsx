import { forwardRef, type HTMLAttributes } from 'react'
import { twMerge } from '@/shared/lib'

/** Neutral by default; a STATUS chip takes the tone that matches its meaning. Colour is never the
 *  only signal — the chip always carries its text label too (THEME-18, THEME-29). `done` is
 *  the solid success plane for a state finished for good, set apart from the tinted `success`. */
export type BadgeTone = 'neutral' | 'accent' | 'danger' | 'success' | 'done' | 'warning' | 'info'

const TONE_STYLES: Record<BadgeTone, string> = {
  neutral: 'bg-badge-neutral-bg text-badge-neutral-fg',
  accent: 'bg-badge-accent-bg text-badge-accent-fg',
  danger: 'bg-notice-danger-bg text-notice-danger-fg',
  success: 'bg-notice-success-bg text-notice-success-fg',
  done: 'bg-badge-done-bg text-badge-done-fg',
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
        // the THEME-23 ratio for a box this small. The label is a STATE (초안 · 검토 · 확정), the
        // one word a row is scanned for, so it reads a step above the 13px `meta` beside it; at
        // 12px it was the smallest text on the card.
        'inline-flex shrink-0 items-center rounded-sm px-2 py-0.5 text-sm font-semibold whitespace-nowrap',
        TONE_STYLES[tone],
        className,
      )}
      {...props}
    />
  )
})
