import { Slider } from './Slider'

/** Two native, labelled handles with identical pointer, touch and keyboard support.
 * Separate hit areas avoid one handle intercepting the other's touch target. */
export function RangeSlider({
  startLabel,
  endLabel,
  value,
  min,
  max,
  step,
  disabled,
  format,
  onChange,
  onCommit,
}: {
  startLabel: string
  endLabel: string
  value: readonly [number, number]
  min: number
  max: number
  step: number
  disabled?: boolean
  format: (value: number) => string
  onChange: (value: [number, number]) => void
  onCommit: () => void
}) {
  return (
    <fieldset
      disabled={disabled}
      className="grid min-w-0 gap-2 sm:grid-cols-2"
      onPointerUp={onCommit}
      onPointerCancel={onCommit}
      onKeyUp={onCommit}
      onBlur={onCommit}
    >
      <Slider
        label={startLabel}
        value={Number.isFinite(value[0]) ? value[0] : min}
        min={min}
        max={max}
        step={step}
        valueText={format(value[0])}
        onChange={(start) => onChange([start, value[1]])}
      />
      <Slider
        label={endLabel}
        value={Number.isFinite(value[1]) ? value[1] : max}
        min={min}
        max={max}
        step={step}
        valueText={format(value[1])}
        onChange={(end) => onChange([value[0], end])}
      />
    </fieldset>
  )
}
