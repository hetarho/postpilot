import { type KeyboardEvent } from 'react'
import { clsx } from 'clsx'
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
  /** id of the `role="tabpanel"` these tabs drive, when they are a tab row rather than a plain
   *  switch. Without it the panel is an orphan in the accessibility tree and a screen-reader user
   *  gets no signal that the panel changed. */
  controls?: string
  /** Refuses every switch while a change is in flight. The whole control greys, because a bounded
   *  switch has no single "pending" option to grey on its own. */
  disabled?: boolean
  className?: string
}

/** The primitive for a bounded switch or a tab row. A slice never hand-rolls `role="tablist"`
 *  (design-language §1.1, §7).
 *
 *  It scrolls horizontally rather than crushing or wrapping its labels: a Korean option set
 *  outgrows the width long before an English one does — four format names alone measure ~380px
 *  against 328px of content at 360px — and a clipped tab looks like a feature that does not exist.
 *  `flex-1` keeps the tabs sharing the width evenly whenever they do fit; `shrink-0` plus
 *  `basis-auto` lets them take their natural width and scroll once they do not. */
export function SegmentedControl<T extends string>({
  value,
  options,
  onChange,
  ariaLabel,
  controls,
  disabled = false,
  className,
}: SegmentedControlProps<T>) {
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (disabled) return
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
      // `overscroll-x-contain`: a swipe that reaches the end of the strip must not chain to the
      // page or to the browser's back gesture (§4.4).
      className={twMerge(
        clsx(
          'bg-surface-recessed flex min-h-11 gap-1 overflow-x-auto overscroll-x-contain rounded-md p-1 select-none',
          disabled && 'opacity-50',
        ),
        className,
      )}
    >
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          role="tab"
          aria-selected={option.value === value}
          aria-controls={controls}
          disabled={disabled}
          tabIndex={option.value === value ? 0 : -1}
          onClick={() => onChange(option.value)}
          // No `focus-visible:ring-*` here: the global `:focus-visible` outline in
          // app/styles/index.css is the app's one focus indicator (§9), and a second ring stacked
          // inside a `p-1` container paints across the neighbouring tabs.
          className={twMerge(
            'min-h-11 flex-1 shrink-0 rounded-sm px-4 text-sm whitespace-nowrap transition-colors',
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
