import { useId } from 'react'
import { twMerge } from 'tailwind-merge'

/** A bounded number the reader adjusts.
 *
 *  It wraps the native range input rather than drawing its own track: pointer dragging, arrow
 *  and Home/End keys, the touch target's own hit slop and every platform's assistive
 *  behaviour come with the element, and none of it is worth reimplementing for a control that
 *  reports an estimate.
 *
 *  The value is text beside the label, not only a thumb position: a slider's position is a
 *  reading nobody can transcribe, and `aria-valuetext` carries the same formatted string so
 *  the two never disagree. */
export function Slider({
  label,
  value,
  min,
  max,
  step,
  valueText,
  onChange,
  className,
}: {
  label: string
  value: number
  min: number
  max: number
  step: number
  /** What the reader sees: already formatted with its unit. */
  valueText: string
  onChange: (value: number) => void
  className?: string
}) {
  const labelId = useId()
  const inputId = `${labelId}-input`

  return (
    <div className={twMerge('min-w-0', className)}>
      <div className="flex items-baseline justify-between gap-2">
        <label id={labelId} htmlFor={inputId} className="text-field-label text-sm">
          {label}
        </label>
        <span className="text-content-primary text-sm font-medium tabular-nums">{valueText}</span>
      </div>
      <input
        id={inputId}
        type="range"
        value={value}
        min={min}
        max={max}
        step={step}
        aria-labelledby={labelId}
        aria-valuetext={valueText}
        onChange={(event) => onChange(Number(event.target.value))}
        // The thumb and track are drawn by the platform from these two roles, and the 44px
        // floor is met by the control's own height rather than by the visible bar (§4.1).
        className="accent-slider-thumb bg-slider-track mt-2 h-11 w-full cursor-pointer appearance-auto rounded-full"
      />
    </div>
  )
}
