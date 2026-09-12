import { FieldLabel, FieldMessage, TextField } from '@/shared/ui'

export function ClipTimeField({
  id,
  label,
  value,
  onChange,
  error,
}: {
  id: string
  label: string
  value: number
  onChange: (ms: number) => void
  error?: string
}) {
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <TextField
        id={id}
        type="number"
        inputMode="decimal"
        step="0.001"
        autoComplete="off"
        value={Number.isFinite(value) ? value / 1000 : ''}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : undefined}
        onChange={(event) =>
          onChange(
            event.target.value === ''
              ? NaN
              : Number((Number(event.target.value) * 1000).toFixed(6)),
          )
        }
      />
      {error && <FieldMessage id={`${id}-error`}>{error}</FieldMessage>}
    </div>
  )
}
