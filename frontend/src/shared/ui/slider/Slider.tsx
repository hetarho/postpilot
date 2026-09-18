import { useId } from 'react'
import { twMerge } from 'tailwind-merge'

interface SliderBase {
  value: number
  min: number
  max: number
  step: number
  /** What the reader sees: already formatted with its unit. */
  valueText: string
  onChange: (value: number) => void
  className?: string
}
/** Either a visible label above the track, or a name given to the control alone.
 *  Never both, and never neither: a slider with no name is unreachable. */
type SliderProps = SliderBase &
  ({ label: string; ariaLabel?: never } | { ariaLabel: string; label?: never })

/** A bounded number the reader adjusts.
 *
 *  It wraps the native range input rather than drawing its own track: pointer dragging, arrow
 *  and Home/End keys, the touch target's own hit slop and every platform's assistive
 *  behaviour come with the element, and none of it is worth reimplementing for a control that
 *  reports an estimate.
 *
 *  The value is text beside the label, not only a thumb position: a slider's position is a
 *  reading nobody can transcribe, and `aria-valuetext` carries the same formatted string so
 *  the two never disagree.
 *
 *  `ariaLabel` in place of `label` takes the label LINE away and leaves one row: the track with
 *  its reading beside it, named through the control itself. That is the shape a scrubber under a
 *  video wants (CLIP-53) — the label said what the surface it sits under already says, and a
 *  second line of it pushed the editor down a row on every phone. */
export function Slider({
  label,
  ariaLabel,
  value,
  min,
  max,
  step,
  valueText,
  onChange,
  className,
}: SliderProps) {
  const labelId = useId()
  const inputId = `${labelId}-input`

  return (
    <div className={twMerge('min-w-0', label ? undefined : 'flex items-center gap-3', className)}>
      {label && (
        <div className="flex items-baseline justify-between gap-2">
          <label id={labelId} htmlFor={inputId} className="text-field-label text-sm">
            {label}
          </label>
          <span className="text-content-primary text-sm font-medium tabular-nums">{valueText}</span>
        </div>
      )}
      <input
        id={inputId}
        type="range"
        value={value}
        min={min}
        max={max}
        step={step}
        aria-label={label ? undefined : ariaLabel}
        aria-labelledby={label ? labelId : undefined}
        aria-valuetext={valueText}
        onChange={(event) => onChange(Number(event.target.value))}
        // The thumb and track are drawn by the platform from these two roles, and the 44px
        // floor is met by the control's own height rather than by the visible bar (§4.1).
        className={twMerge(
          'accent-slider-thumb bg-slider-track h-11 w-full cursor-pointer appearance-auto rounded-full',
          label && 'mt-2',
        )}
      />
      {/* One row: the reading sits BESIDE the track rather than above it, so the scrubber
          states its own second without a line of its own. */}
      {!label && (
        <span className="text-content-primary shrink-0 text-sm font-medium tabular-nums">
          {valueText}
        </span>
      )}
    </div>
  )
}
