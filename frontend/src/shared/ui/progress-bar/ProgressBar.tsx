import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'

/** How far a job that is running RIGHT NOW has got, as a hairline track.
 *
 *  It is a `role="progressbar"` and not the existing `Meter`: a meter reports a level within a
 *  range that resets and always renders its figures as the primary reading, while this moves once
 *  toward completion and then goes away. The figures are deliberately absent — the words beside
 *  the bar name the stage, and the numbers are the bar (design-language §4.3).
 *
 *  A stage that reports no ratio takes the SAME track in an indeterminate state rather than a
 *  different control, so the place on the screen that means "something is running" never moves.
 *
 *  It occupies NO layout height: the root is a zero-height positioning context and the track is
 *  painted out of flow, so a bar appearing mid-scroll cannot push the draft under the reader's
 *  thumb, and its departure cannot leave a gap. Where it sits is the caller's decision. */
export function ProgressBar({
  label,
  done,
  total,
  className,
}: {
  /** The bar's accessible name. It carries no visible text of its own. */
  label: string
  /** Determinate only when both are given and `total` is positive; indeterminate otherwise. */
  done?: number
  total?: number
  /** Placement, on the zero-height root. */
  className?: string
}) {
  const determinate = done !== undefined && total !== undefined && total > 0
  const filled = determinate ? Math.min(1, Math.max(0, done / total)) : 0
  return (
    <div className={twMerge('relative h-0', className)}>
      <div
        role="progressbar"
        aria-label={label}
        // No `aria-valuenow` at all while indeterminate — that is what the attribute's absence
        // MEANS, and a 0 would announce a job that has stalled at the start.
        // Bounded by the range it is announced AGAINST, not just where it is painted: a job
        // projection that reports more done than total would otherwise announce a value outside
        // its own min/max.
        aria-valuenow={determinate ? Math.min(total, Math.max(0, done)) : undefined}
        aria-valuemin={determinate ? 0 : undefined}
        aria-valuemax={determinate ? total : undefined}
        className="bg-meter-track h-progress-bar absolute inset-x-0 top-0 overflow-hidden"
      >
        <div
          className={clsx(
            'bg-meter-fill h-full',
            determinate
              ? 'duration-base ease-standard transition-[inline-size]'
              : 'animate-progress-sweep w-1/5',
          )}
          style={determinate ? { inlineSize: `${filled * 100}%` } : undefined}
        />
      </div>
    </div>
  )
}
