import type { ComponentType } from 'react'
import { clsx } from 'clsx'
import { BookMarked, Check, LayoutTemplate, ListChecks, Plus, Quote, Sparkles } from 'lucide-react'
import type { KitKey, KitState } from '../model/kit'

const KIT_ICONS: Record<KitKey, ComponentType<{ className?: string }>> = {
  voice: Quote,
  template: LayoutTemplate,
  guideline: ListChecks,
  ai: Sparkles,
  memory: BookMarked,
}

const RADIUS = 24
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

/** One piece of the kit as a ring: drawn in full when it is ready, as far as its own meter has
 *  come while it is being made, and as a dashed outline with a plus while it is missing. The ring
 *  is decoration; the state is said in words by whatever holds it. */
export function KitRing({
  kitKey,
  state,
  fill,
  size = 'md',
}: {
  kitKey: KitKey
  state: KitState
  fill: number
  size?: 'sm' | 'md'
}) {
  const Icon = KIT_ICONS[kitKey]
  const ready = state === 'ready'
  const missing = state === 'empty'
  return (
    <span
      aria-hidden="true"
      className={clsx(
        'relative inline-flex shrink-0 items-center justify-center',
        size === 'md' ? 'size-14' : 'size-10',
      )}
    >
      <svg viewBox="0 0 56 56" className="absolute inset-0 size-full -rotate-90">
        <circle
          cx="28"
          cy="28"
          r={RADIUS}
          fill="none"
          strokeWidth="4"
          strokeDasharray={missing ? '3 5' : undefined}
          className={missing ? 'stroke-content-tertiary' : 'stroke-progress-step-todo'}
        />
        {state !== 'empty' && state !== 'loading' && (
          <circle
            cx="28"
            cy="28"
            r={RADIUS}
            fill="none"
            strokeWidth="4"
            strokeLinecap="round"
            strokeDasharray={`${Math.max(0.001, fill) * CIRCUMFERENCE} ${CIRCUMFERENCE}`}
            className="stroke-meter-fill duration-slow ease-emphasized transition-[stroke-dasharray]"
          />
        )}
      </svg>
      <span
        className={clsx(
          'flex items-center justify-center rounded-full',
          size === 'md' ? 'size-10' : 'size-7',
          ready
            ? 'bg-button-cta-bg text-button-cta-fg'
            : state === 'partial'
              ? 'bg-badge-accent-bg text-badge-accent-fg'
              : 'bg-surface-recessed text-content-tertiary',
          state === 'loading' && 'loading-skeleton',
        )}
      >
        <Icon className={size === 'md' ? 'size-5' : 'size-4'} />
      </span>
      {(ready || missing) && size === 'md' && (
        <span
          className={clsx(
            'ring-surface-base absolute -right-0.5 -bottom-0.5 flex size-5 items-center justify-center rounded-full ring-2',
            ready
              ? 'bg-badge-done-bg text-badge-done-fg'
              : 'bg-surface-raised text-content-secondary',
          )}
        >
          {ready ? <Check className="size-3" /> : <Plus className="size-3" />}
        </span>
      )}
    </span>
  )
}
