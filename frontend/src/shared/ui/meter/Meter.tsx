import { useId } from 'react'
import { twMerge } from 'tailwind-merge'

/** How much of a bounded allowance has been used.
 *
 *  It is a `role="meter"` and not a progress bar: progress moves toward completion, while this
 *  reports a level within a range that resets. The figures beside the label are the primary
 *  reading — the bar only makes "nearly full" visible at a glance, so colour is never the only
 *  signal (design-language §2.6).
 *
 *  An unlimited allowance (`max` of 0) renders the label and figures with no bar at all: an empty
 *  track next to "unlimited" reads as "none left". */
export function Meter({
  label,
  value,
  max,
  valueText,
  note,
  className,
}: {
  label: string
  value: number
  max: number
  /** What the user reads: already formatted for money, counts, or "unlimited". */
  valueText: string
  note?: string
  className?: string
}) {
  const labelId = useId()
  const bounded = max > 0 ? Math.min(1, Math.max(0, value / max)) : 0
  return (
    <div className={twMerge('min-w-0', className)}>
      <div className="flex items-baseline justify-between gap-2">
        <span id={labelId} className="text-content-secondary text-xs">
          {label}
        </span>
        <span className="text-content-primary text-xs font-medium tabular-nums">{valueText}</span>
      </div>
      {max > 0 && (
        <div
          role="meter"
          aria-labelledby={labelId}
          aria-valuenow={value}
          aria-valuemin={0}
          aria-valuemax={max}
          aria-valuetext={valueText}
          className="bg-meter-track mt-1 h-1.5 w-full overflow-hidden rounded-full"
        >
          <div
            className="bg-meter-fill h-full rounded-full"
            style={{ inlineSize: `${bounded * 100}%` }}
          />
        </div>
      )}
      {note && <p className="text-content-tertiary mt-1 text-xs">{note}</p>}
    </div>
  )
}
