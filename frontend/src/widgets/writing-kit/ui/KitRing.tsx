import type { CSSProperties } from 'react'
import { clsx } from 'clsx'
import { twMerge } from '@/shared/lib'
import { Check, Plus } from 'lucide-react'
import {
  aiIllustration,
  guidelineIllustration,
  memoryIllustration,
  templateIllustration,
  voiceIllustration,
} from '@/shared/assets'
import type { KitKey, KitState } from '../model/kit'

/** Each piece of the kit is drawn as its own clay object — the same one its screen opens with. */
const KIT_ILLUSTRATIONS: Record<KitKey, string> = {
  voice: voiceIllustration,
  template: templateIllustration,
  guideline: guidelineIllustration,
  ai: aiIllustration,
  memory: memoryIllustration,
}

/** Three sizes of the same piece: `sm` beside a line of text, `md` on 홈's orbit, `lg` at the
 *  head of the piece's own screen. The stroke is set per size so the ring reads equally thin at
 *  every one of them. */
const SIZES = {
  sm: { box: 'size-11', stroke: 7, badge: undefined },
  md: { box: 'size-16', stroke: 5, badge: 'size-5 [&_svg]:size-3' },
  lg: { box: 'size-24 sm:size-32', stroke: 3.5, badge: 'size-7 sm:size-8 [&_svg]:size-4' },
} as const

/** One piece of the kit: its illustration on a disc inside a ring. The ring is drawn in full when
 *  the piece is ready, as far as its own meter has come while it is being made, and as a dashed
 *  outline while it is missing — when the object itself is greyed out, waiting to be made. A ready
 *  piece carries a check, a missing one a plus.
 *
 *  Decoration only: whatever holds the ring says the piece's state in words. */
export function KitRing({
  kitKey,
  state,
  fill,
  size = 'md',
  className,
}: {
  kitKey: KitKey
  state: KitState
  fill: number
  size?: keyof typeof SIZES
  /** Extra sizing, e.g. a larger ring where its container has room. */
  className?: string
}) {
  const { box, stroke, badge } = SIZES[size]
  const ready = state === 'ready'
  const missing = state === 'empty'
  // A piece that could not be read is drawn faded on a plain track: not ready, not missing either.
  const faded = missing || state === 'failed'
  const radius = 50 - stroke / 2 - 0.5
  const circumference = 2 * Math.PI * radius
  return (
    <span
      aria-hidden="true"
      className={twMerge(clsx('relative inline-flex shrink-0 rounded-full', box, className))}
    >
      <svg viewBox="0 0 100 100" className="absolute inset-0 size-full -rotate-90">
        <circle
          cx="50"
          cy="50"
          r={radius}
          fill="none"
          strokeWidth={stroke}
          strokeDasharray={missing ? `${stroke * 0.9} ${stroke * 1.6}` : undefined}
          className={missing ? 'stroke-content-tertiary' : 'stroke-progress-step-todo'}
        />
        {(ready || state === 'partial') && (
          <circle
            cx="50"
            cy="50"
            r={radius}
            fill="none"
            strokeWidth={stroke}
            strokeLinecap="round"
            strokeDasharray={`${Math.max(0.001, fill) * circumference} ${circumference}`}
            // Drawn in once as the piece arrives, then it only moves when its own fill does.
            style={{ '--ring-length': Math.max(0.001, fill) * circumference } as CSSProperties}
            className="stroke-meter-fill duration-slow ease-emphasized animate-ring-draw transition-[stroke-dasharray]"
          />
        )}
      </svg>
      <span
        className={clsx(
          'absolute inset-[11%] rounded-full',
          faded || state === 'loading' ? 'bg-surface-recessed' : 'bg-surface-raised shadow-sm',
          state === 'loading' && 'loading-skeleton',
        )}
      />
      {state !== 'loading' && (
        <img
          src={KIT_ILLUSTRATIONS[kitKey]}
          alt=""
          className={clsx(
            'duration-slow ease-standard absolute inset-[17%] size-[66%] object-contain transition-[filter,opacity]',
            faded && 'opacity-45 grayscale',
          )}
        />
      )}
      {badge && (ready || missing) && (
        <span
          className={clsx(
            'ring-surface-base absolute right-[2%] bottom-[2%] flex items-center justify-center rounded-full ring-2',
            badge,
            ready
              ? 'bg-badge-done-bg text-badge-done-fg'
              : 'bg-surface-raised text-content-secondary shadow-sm',
          )}
        >
          {ready ? <Check /> : <Plus />}
        </span>
      )}
    </span>
  )
}
