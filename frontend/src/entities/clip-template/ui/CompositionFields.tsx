import { useId } from 'react'
import { FieldLabel, Listbox, TextField, Textarea } from '@/shared/ui'

export function CompositionInput({
  label,
  value,
  onChange,
  multiline = false,
  numeric = false,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  multiline?: boolean
  numeric?: boolean
}) {
  const id = useId()
  return (
    <div className="min-w-0">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {multiline ? (
        <Textarea id={id} value={value} autoGrow onChange={(e) => onChange(e.target.value)} />
      ) : (
        <TextField
          id={id}
          value={value}
          inputMode={numeric ? 'decimal' : 'text'}
          autoComplete="off"
          onChange={(e) => onChange(e.target.value)}
        />
      )}
    </div>
  )
}
export function CompositionSelect({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: { value: string; label: string }[]
  onChange: (value: string) => void
}) {
  const id = useId()
  return (
    <div className="min-w-0">
      <FieldLabel id={`${id}-label`} htmlFor={id}>
        {label}
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={`${id}-label`}
        value={value}
        options={options}
        onChange={onChange}
      />
    </div>
  )
}
