import type { ReactNode } from 'react'
import { clsx } from 'clsx'
import { KIT_LOOK } from '../config/kit'
import { KIT_MAX_MARKS, type KitReading } from '../model/readiness'

const RADIUS = 42
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

/** The ring a kit element is drawn as: one mark per thing the owner made, in the element's hue,
 *  and what stands in for it (a draft, a 기본 지침) as a quieter mark; a voice still being made
 *  fills by its readiness share; an element that holds nothing is a dashed outline. Decorative —
 *  the words beside it carry the state. */
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
  const stroke = size === 'sm' ? 12 : 8
  const { marks } = reading
  const empty =
    !reading.pending &&
    (marks.kind === 'share' ? marks.percent === 0 : marks.own + marks.assumed === 0)
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
          className={clsx(empty ? 'stroke-content-tertiary' : 'stroke-meter-track')}
          strokeDasharray={empty ? '6 8' : undefined}
          strokeLinecap="round"
        />
        {!reading.pending && marks.kind === 'share' && marks.percent > 0 && (
          <circle
            cx="50"
            cy="50"
            r={RADIUS}
            fill="none"
            strokeWidth={stroke}
            className={look.stroke}
            strokeOpacity={reading.state === 'ready' ? 1 : 0.4}
            strokeDasharray={`${(CIRCUMFERENCE * marks.percent) / 100} ${CIRCUMFERENCE}`}
            strokeLinecap="round"
          />
        )}
        {!reading.pending &&
          marks.kind === 'count' &&
          segments(marks.own, marks.assumed).map((segment, index) => (
            <path
              key={index}
              d={arc(segment.from, segment.to)}
              fill="none"
              strokeWidth={stroke}
              strokeLinecap="round"
              className={segment.own ? look.stroke : 'stroke-content-tertiary'}
              strokeOpacity={segment.own ? 1 : 0.55}
            />
          ))}
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

/** The marks around the ring, the owner's own first: a gap between marks keeps each one a thing
 *  that can be counted, and past `KIT_MAX_MARKS` the ring closes. */
function segments(own: number, assumed: number) {
  const total = Math.min(KIT_MAX_MARKS, own + assumed)
  if (total === 0) return []
  const ownShown = Math.min(own, total)
  if (total === 1) return [{ from: 0, to: 359.9, own: ownShown === 1 }]
  const gap = total > 8 ? 10 : 16
  const span = 360 / total
  return Array.from({ length: total }, (_, index) => ({
    from: index * span + gap / 2,
    to: (index + 1) * span - gap / 2,
    own: index < ownShown,
  }))
}

function arc(from: number, to: number): string {
  const point = (degrees: number) => {
    const radians = (degrees * Math.PI) / 180
    return `${50 + RADIUS * Math.cos(radians)} ${50 + RADIUS * Math.sin(radians)}`
  }
  const large = to - from > 180 ? 1 : 0
  return `M ${point(from)} A ${RADIUS} ${RADIUS} 0 ${large} 1 ${point(to)}`
}
