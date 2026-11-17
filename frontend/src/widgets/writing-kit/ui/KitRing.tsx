import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { KIT_LOOK } from '../config/kit'
import type { KitReading } from '../model/readiness'

const RADIUS = 42
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

/** One kit element as a small ring, drawn the way its quarter of the home graphic is: whole and
 *  in the element's hue once ready, a quieter hue while only stand-ins hold it (a voice still
 *  being made fills only as far as its readiness share), a dashed outline while empty.
 *  Decorative — the words beside it carry the state. */
export function KitRing({
  reading,
  size = 'lg',
  children,
  className,
}: {
  reading: KitReading
  size?: 'sm' | 'md' | 'lg'
  children?: ReactNode
  className?: string
}) {
  const look = KIT_LOOK[reading.key]
  const Icon = look.icon
  const stroke = size === 'sm' ? 13 : 9
  const settled = !reading.pending && !reading.failed
  const empty = settled && reading.state === 'empty'
  const drawn =
    !settled || reading.state === 'empty'
      ? 0
      : reading.state === 'ready' || reading.share === 0
        ? 100
        : reading.share
  return (
    <span
      aria-hidden="true"
      className={clsx(
        'relative inline-flex shrink-0 items-center justify-center rounded-full',
        size === 'sm' ? 'size-5' : size === 'md' ? 'size-12' : 'size-16 sm:size-20',
        className,
      )}
    >
      <svg viewBox="0 0 100 100" className="absolute inset-0 size-full -rotate-90">
        <circle
          cx="50"
          cy="50"
          r={RADIUS}
          fill="none"
          strokeWidth={stroke}
          className={empty ? 'stroke-content-tertiary' : 'stroke-meter-track'}
          strokeDasharray={empty ? '7 9' : undefined}
        />
        {drawn > 0 && (
          <circle
            cx="50"
            cy="50"
            r={RADIUS}
            fill="none"
            strokeWidth={stroke}
            className={look.stroke}
            strokeOpacity={reading.state === 'ready' ? 1 : 0.45}
            strokeDasharray={
              drawn >= 100 ? undefined : `${(CIRCUMFERENCE * drawn) / 100} ${CIRCUMFERENCE}`
            }
            strokeLinecap={drawn >= 100 ? undefined : 'round'}
          />
        )}
      </svg>
      {children ??
        (size !== 'sm' && (
          <Icon
            className={clsx(
              'relative',
              size === 'md' ? 'size-5' : 'size-6 sm:size-7',
              empty ? 'text-content-tertiary' : look.text,
            )}
          />
        ))}
    </span>
  )
}
