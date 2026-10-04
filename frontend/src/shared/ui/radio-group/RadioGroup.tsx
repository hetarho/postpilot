import { useId, useRef, type ReactNode } from 'react'
export interface RadioOption {
  value: string
  label: string
  detail?: ReactNode
  disabled?: boolean
}
export function RadioGroup({
  label,
  value,
  options,
  onChange,
  disabled = false,
}: {
  label: string
  value: string
  options: readonly RadioOption[]
  onChange: (value: string) => void
  disabled?: boolean
}) {
  const name = useId(),
    elements = useRef(new Map<string, HTMLInputElement>())
  return (
    <fieldset disabled={disabled} className="min-w-0">
      <legend className="sr-only">{label}</legend>
      <div className="divide-divider divide-y">
        {options.map((option, i) => (
          <div key={option.value} className="min-w-0 py-4">
            <label className="active:bg-row-bg-active flex min-h-11 min-w-0 cursor-pointer items-center gap-3 rounded-md px-3 font-medium">
              <input
                type="radio"
                name={name}
                value={option.value}
                checked={value === option.value}
                disabled={option.disabled}
                ref={(element) => {
                  if (element) elements.current.set(option.value, element)
                  else elements.current.delete(option.value)
                }}
                onChange={() => onChange(option.value)}
                onKeyDown={(event) => {
                  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(event.key))
                    return
                  event.preventDefault()
                  const direction = ['ArrowLeft', 'ArrowUp'].includes(event.key) ? -1 : 1
                  for (let step = 1; step <= options.length; step++) {
                    const next = options[(i + direction * step + options.length) % options.length]
                    if (next && !next.disabled) {
                      onChange(next.value)
                      elements.current.get(next.value)?.focus()
                      break
                    }
                  }
                }}
                className="accent-intent-accent size-4 shrink-0"
              />
              <span className="min-w-0 break-words">{option.label}</span>
            </label>
            {option.detail && <div className="mt-2 px-3">{option.detail}</div>}
          </div>
        ))}
      </div>
    </fieldset>
  )
}
