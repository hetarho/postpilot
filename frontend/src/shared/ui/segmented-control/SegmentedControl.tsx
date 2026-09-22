import {
  Fragment,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from 'react'
import { clsx } from 'clsx'
import { ChevronRight } from 'lucide-react'
import { twMerge } from 'tailwind-merge'

export interface SegmentedOption<T extends string> {
  value: T
  label: string
  preview?: ReactNode
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
  /** `segments` is the pill row on its recessed plane — a switch or a tab row standing in the
   *  flow. `steps` is the same control drawn as a lifecycle's stations: the names as text with a
   *  chevron between them, the current one told by colour alone and no plane under any of them —
   *  for a step bar that rides a page's top row between two controls, where a row of pills read
   *  as three more buttons (THEME-39, owner decision 2026-09-19). */
  variant?: 'segments' | 'steps'
  /** `default` is the standalone control, at the 40px fine-pointer row and the 44px touch floor.
   *  `compact` is the same control inside a surface that is itself a control — the preferences in
   *  the account panel — where a full-height row reads as three buttons stacked in a popover
   *  rather than as one setting (owner decision 2026-09-22). It keeps a 32px row and a 36px touch
   *  floor: still a comfortable target for a switch whose options sit side by side. */
  size?: 'default' | 'compact'
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
  variant = 'segments',
  size = 'default',
  className,
}: SegmentedControlProps<T>) {
  const steps = variant === 'steps'
  const compact = size === 'compact'
  // The selected plane is ONE element that TRAVELS between the options instead of a background
  // that blinks from one to the next: the switch then shows which way the choice moved, which is
  // the point of laying the options out side by side (owner decision 2026-09-22).
  //
  // Measured here and moved by a CSS transition, rather than by a layout animation: the options
  // are not always equal width — a long label makes the strip scroll — so the position cannot be
  // computed from the index, and a per-frame layout animation on a control this common made the
  // clip workspace's suites time out. A reduced-motion request is already honoured by the global
  // rule in app/styles/index.css, which is another thing a CSS transition gets for free.
  const listRef = useRef<HTMLDivElement>(null)
  const [thumb, setThumb] = useState<{ left: number; width: number }>()
  const shape = options.map((option) => option.value).join('|')
  useLayoutEffect(() => {
    if (steps) return
    const measure = () => {
      const selected = listRef.current?.querySelector<HTMLElement>(
        '[role="tab"][aria-selected="true"]',
      )
      if (!selected) return setThumb(undefined)
      const next = { left: selected.offsetLeft, width: selected.offsetWidth }
      // Same box, same object: a new one every measure would restart this effect forever.
      setThumb((previous) =>
        previous && previous.left === next.left && previous.width === next.width ? previous : next,
      )
    }
    measure()
    window.addEventListener('resize', measure)
    return () => window.removeEventListener('resize', measure)
  }, [steps, value, shape, compact])
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
      ref={listRef}
      role="tablist"
      aria-label={ariaLabel}
      onKeyDown={onKeyDown}
      // `overscroll-x-contain`: a swipe that reaches the end of the strip must not chain to the
      // page or to the browser's back gesture (§4.4).
      className={twMerge(
        clsx(
          steps
            ? 'flex min-h-10 items-center justify-center gap-1 select-none pointer-coarse:min-h-11'
            : 'bg-surface-recessed relative flex gap-1 overflow-x-auto overscroll-x-contain rounded-md p-1 select-none',
          !steps &&
            (compact ? 'min-h-8 pointer-coarse:min-h-9' : 'min-h-10 pointer-coarse:min-h-11'),
          disabled && 'opacity-50',
        ),
        className,
      )}
    >
      {/* The travelling plane, once, under every option: one element that moves is what says the
          choice slid left or right. It is absent until the first measure, so a control rendered
          where there is no layout — a test environment, a hidden panel — simply has none. */}
      {!steps && thumb && (
        <span
          aria-hidden="true"
          style={{ transform: `translateX(${thumb.left}px)`, width: thumb.width }}
          className="bg-surface-raised duration-base ease-standard absolute top-1 bottom-1 left-0 rounded-sm shadow-sm transition-[transform,width]"
        />
      )}
      {options.map((option, index) => (
        <Fragment key={option.value}>
          {/* The path between stations. Decoration: the tabs' order already says it. */}
          {steps && index > 0 && (
            <ChevronRight aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
          )}
          <button
            type="button"
            role="tab"
            aria-selected={option.value === value}
            aria-label={option.preview ? option.label : undefined}
            aria-controls={controls}
            disabled={disabled}
            tabIndex={
              option.value === value || (!options.some((o) => o.value === value) && index === 0)
                ? 0
                : -1
            }
            onClick={() => onChange(option.value)}
            // No `focus-visible:ring-*` here: the global `:focus-visible` outline in
            // app/styles/index.css is the app's one focus indicator (§9), and a second ring stacked
            // inside a `p-1` container paints across the neighbouring tabs.
            className={twMerge(
              steps
                ? clsx(
                    'inline-flex min-h-10 shrink-0 items-center rounded-sm px-2 text-sm whitespace-nowrap transition-colors pointer-coarse:min-h-11',
                    option.value === value
                      ? 'text-content-primary font-medium'
                      : 'text-content-tertiary hover:text-content-secondary active:text-content-secondary',
                  )
                : clsx(
                    'relative flex-1 shrink-0 rounded-sm whitespace-nowrap transition-colors',
                    compact
                      ? 'min-h-8 px-2 text-xs pointer-coarse:min-h-9'
                      : 'min-h-10 px-4 text-sm pointer-coarse:min-h-11',
                    option.value === value
                      ? 'text-content-primary'
                      : 'text-content-secondary hover:bg-row-bg-hover active:bg-row-bg-active',
                  ),
            )}
          >
            <span className="inline-flex items-center justify-center gap-1">
              {option.preview}
              {option.label}
            </span>
          </button>
        </Fragment>
      ))}
    </div>
  )
}
