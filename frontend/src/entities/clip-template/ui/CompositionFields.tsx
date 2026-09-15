import { useId } from 'react'
import { FieldLabel, Listbox, TextField, Textarea, Typography } from '@/shared/ui'

export function CompositionInput({
  label,
  value,
  onChange,
  multiline = false,
  numeric = false,
  hint,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  multiline?: boolean
  numeric?: boolean
  /** What the position allows, stated under the control so the author sees the
   *  ceiling before typing a number rather than after the parser refuses it. */
  hint?: string
}) {
  const id = useId()
  const hintId = `${id}-hint`
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
          aria-describedby={hint ? hintId : undefined}
          onChange={(e) => onChange(e.target.value)}
        />
      )}
      {hint && (
        <Typography id={hintId} variant="meta" as="p" className="text-content-secondary mt-2">
          {hint}
        </Typography>
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
