import { type KeyboardEvent } from 'react'
import { twMerge } from 'tailwind-merge'

export interface SegmentedOption<T extends string> {
  value: T
  label: string
}

interface SegmentedControlProps<T extends string> {
  value: T
  options: readonly SegmentedOption<T>[]
  onChange: (value: T) => void
  ariaLabel: string
  className?: string
}

export function SegmentedControl<T extends string>({
  value,
  options,
  onChange,
  ariaLabel,
  className,
}: SegmentedControlProps<T>) {
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
    event.preventDefault()
    const current = options.findIndex((option) => option.value === value)
    const delta = event.key === 'ArrowRight' ? 1 : -1
    const next = (current + delta + options.length) % options.length
    onChange(options[next].value)
    event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]')[next]?.focus()
  }
  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      onKeyDown={onKeyDown}
      className={twMerge(
        'bg-surface-recessed grid min-h-11 grid-flow-col rounded-md p-1',
        className,
      )}
    >
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="tab"
          aria-selected={option.value === value}
          tabIndex={option.value === value ? 0 : -1}
          onClick={() => onChange(option.value)}
          className={twMerge(
            'focus-visible:ring-focus-ring min-h-11 rounded-sm px-4 text-sm transition-colors focus-visible:ring-2',
            option.value === value
              ? 'bg-surface-raised text-content-primary shadow-sm'
              : 'text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  )
}
