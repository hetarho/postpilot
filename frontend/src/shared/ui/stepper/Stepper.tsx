import { useId, type KeyboardEvent } from 'react'
import { Minus, Plus } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import { Button } from '../button/Button'

export interface StepperProps {
  label: string
  value: number
  min: number
  max: number
  onChange: (value: number) => void
  disabled?: boolean
  /** Named separately because a glyph is not a name: a screen reader reads the button, not the −. */
  decrementLabel: string
  incrementLabel: string
  /** How the current value reads. Defaults to the bare number. */
  formatValue?: (value: number) => string
  className?: string
}

/** A small bounded integer, changed one step at a time.
 *
 *  Two buttons and a readout rather than a number input: the values are single digits inside a
 *  hard range, and a text field would invite typing one the server refuses — while costing a
 *  numeric keyboard, a selection and a blur on a phone.
 *
 *  The readout is the `spinbutton`, so the value and its bounds are announced together and the
 *  arrow keys work the way they do on a native one. The buttons DISABLE at the bounds rather
 *  than clamping silently: a control that accepts a press and does nothing reads as broken.
 */
export function Stepper({
  label,
  value,
  min,
  max,
  onChange,
  disabled = false,
  decrementLabel,
  incrementLabel,
  formatValue,
  className,
}: StepperProps) {
  const labelId = useId()
  const step = (to: number) => {
    const next = Math.max(min, Math.min(max, to))
    if (next !== value) onChange(next)
  }
  const onKeyDown = (event: KeyboardEvent<HTMLSpanElement>) => {
    if (disabled) return
    if (event.key === 'ArrowUp' || event.key === 'ArrowRight') {
      event.preventDefault()
      step(value + 1)
    }
    if (event.key === 'ArrowDown' || event.key === 'ArrowLeft') {
      event.preventDefault()
      step(value - 1)
    }
    if (event.key === 'Home') {
      event.preventDefault()
      step(min)
    }
    if (event.key === 'End') {
      event.preventDefault()
      step(max)
    }
  }
  return (
    <div className={twMerge('min-w-0', className)}>
      <span id={labelId} className="text-content-secondary text-xs">
        {label}
      </span>
      <div role="group" aria-labelledby={labelId} className="mt-1 flex items-center gap-2">
        <Button
          variant="secondary"
          size="icon"
          aria-label={decrementLabel}
          disabled={disabled || value <= min}
          onClick={() => step(value - 1)}
        >
          <Minus aria-hidden="true" className="size-4" />
        </Button>
        <span
          role="spinbutton"
          tabIndex={disabled ? -1 : 0}
          aria-labelledby={labelId}
          aria-valuenow={value}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-disabled={disabled || undefined}
          onKeyDown={onKeyDown}
          className="text-content-primary min-w-8 rounded-md text-center text-sm font-medium tabular-nums"
        >
          {formatValue ? formatValue(value) : String(value)}
        </span>
        <Button
          variant="secondary"
          size="icon"
          aria-label={incrementLabel}
          disabled={disabled || value >= max}
          onClick={() => step(value + 1)}
        >
          <Plus aria-hidden="true" className="size-4" />
        </Button>
      </div>
    </div>
  )
}
