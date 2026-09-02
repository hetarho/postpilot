import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import { Check, ChevronDown } from 'lucide-react'
import {
  LISTBOX_MAX_VIEWPORT_RATIO,
  LISTBOX_MIN_PANEL_PX,
  LISTBOX_TRIGGER_GAP_PX,
  LISTBOX_VIEWPORT_GUTTER_PX,
} from '@/shared/config'

export interface ListboxOption<T> {
  value: T
  label: string
  /** Listed but not choosable. `aria-disabled`, not `disabled`, so the arrow keys still reach it
   *  and a screen reader still reads the entry whose whole purpose is to say why it is unusable. */
  disabled?: boolean
}

interface ListboxProps<T> {
  /** Goes on the TRIGGER, so a caller's `FieldLabel htmlFor` still points at the control. */
  id?: string
  value: T
  options: readonly ListboxOption<T>[]
  onChange: (value: T) => void
  /** Shown when `value` matches no option — the empty state of a field with no choice yet. */
  placeholder?: string
  disabled?: boolean
  className?: string
  /** id of the visible `FieldLabel`. The trigger's name becomes "<label> <current option>", which
   *  is what a native select announced. */
  'aria-labelledby'?: string
  'aria-label'?: string
  'aria-describedby'?: string
  'aria-invalid'?: boolean
}

/** The app-drawn replacement for the native select element (design-language §7, owner decision
 *  2026-08-31). The OS draws a native select's open option list, so it can wear neither the app's
 *  surfaces nor its type roles and it visibly leaves the design system the moment it opens.
 *
 *  It is NOT `Menu`: that primitive is `aria-haspopup="menu"` behind an icon-only trigger, which
 *  is the wrong role and the wrong shape for a labelled form field.
 *
 *  WAI-APG listbox-button semantics: the trigger is the only tab stop, options are programmatic
 *  focus targets (`tabIndex={-1}`) inside a `role="listbox"` panel, and the check marks the
 *  current value. Options are ordinary buttons, so Enter and Space select through the native
 *  click; Escape and an outside press close and return focus to the trigger.
 *
 *  Generic over the option value rather than over a string, so a numeric enum (a block type, a
 *  heading level) round-trips without a stringly-typed cast at every call site. */
export function Listbox<T>({
  id,
  value,
  options,
  onChange,
  placeholder,
  disabled = false,
  className,
  'aria-labelledby': labelledBy,
  'aria-label': ariaLabel,
  'aria-describedby': describedBy,
  'aria-invalid': invalid,
}: ListboxProps<T>) {
  const generatedId = useId()
  const panelId = `${generatedId}-panel`
  const valueId = `${generatedId}-value`
  const [open, setOpen] = useState(false)
  const [drop, setDrop] = useState<'down' | 'up'>('down')
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  const selected = options.find((option) => option.value === value)

  const items = () =>
    Array.from(panelRef.current?.querySelectorAll<HTMLButtonElement>('[role="option"]') ?? [])

  const close = (returnFocus: boolean) => {
    setOpen(false)
    if (returnFocus) queueMicrotask(() => triggerRef.current?.focus())
  }

  const select = (option: ListboxOption<T>) => {
    if (option.disabled) return
    close(true)
    if (option.value !== value) onChange(option.value)
  }

  // A trigger can sit anywhere in the viewport, and nothing in CSS knows where — a `dvh` ceiling
  // is the same number for a field at the top of the page and for one in the docked bar, where a
  // panel half the screen tall opens entirely past the bottom edge. So the room is MEASURED: the
  // panel opens downward into the gap it actually has, flips above the trigger when that gap is
  // too small and the one overhead is larger, and scrolls inside whichever it took. The ratio
  // caps it where there is more room than a list needs; the floor keeps it usable rather than
  // squeezing it to a sliver for a trigger pinned against an edge.
  useLayoutEffect(() => {
    const panel = panelRef.current
    const trigger = triggerRef.current
    if (!open || !panel || !trigger) return
    const anchor = trigger.getBoundingClientRect()
    const gap = LISTBOX_TRIGGER_GAP_PX + LISTBOX_VIEWPORT_GUTTER_PX
    const below = window.innerHeight - anchor.bottom - gap
    const above = anchor.top - gap
    const flip = below < LISTBOX_MIN_PANEL_PX && above > below
    const room = Math.min(flip ? above : below, window.innerHeight * LISTBOX_MAX_VIEWPORT_RATIO)
    setDrop(flip ? 'up' : 'down')
    panel.style.maxHeight = `${Math.max(room, LISTBOX_MIN_PANEL_PX)}px`
  }, [open, options])

  useEffect(() => {
    if (!open) return
    // The current option receives focus so the keyboard arrives where the state already is —
    // read off the rendered rows so the effect depends on nothing but being open.
    queueMicrotask(() => {
      const elements = items()
      const current = elements.find((element) => element.getAttribute('aria-selected') === 'true')
      ;(current ?? elements[0])?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) close(true)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  const onPanelKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      // The listbox may be open INSIDE an overlay that also closes on Escape (the writing brief's
      // popover, a sheet). One Escape must dismiss one thing — the innermost one.
      event.stopPropagation()
      close(true)
      return
    }
    if (event.key === 'Tab') {
      // Do not prevent the native traversal: options are programmatic stops, so forward Tab
      // reaches the next page control and Shift+Tab reaches the trigger. Keep the focused row
      // mounted until that traversal completes, or the browser restarts from the trigger and the
      // user pays an extra key press.
      setTimeout(() => setOpen(false), 0)
      return
    }
    if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      items()
        .at(event.key === 'Home' ? 0 : -1)
        ?.focus()
      return
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    const elements = items()
    if (!elements.length) return
    const step = event.key === 'ArrowDown' ? 1 : -1
    const current = elements.indexOf(document.activeElement as HTMLButtonElement)
    elements[(current + step + elements.length) % elements.length]?.focus()
  }

  const onTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    setOpen(true)
  }

  return (
    <div ref={rootRef} className={twMerge('relative', className)}>
      <button
        ref={triggerRef}
        type="button"
        id={id}
        disabled={disabled}
        // `combobox`, which is the role a native select exposed, over the plain button a
        // menu-style trigger would be: this is a form field whose value is one of a list, and
        // assistive technology should keep announcing it as one.
        role="combobox"
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        aria-labelledby={labelledBy ? `${labelledBy} ${valueId}` : undefined}
        aria-label={ariaLabel}
        aria-describedby={describedBy}
        aria-invalid={invalid}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onTriggerKeyDown}
        // The field well the native select wore, minus the native control. `px-4` against the
        // ~12px the 44px floor produces is the §4.2 ratio; the type is the `input` role (§3.1).
        className="bg-field-bg text-field-fg hover:bg-field-bg-hover focus:bg-field-bg-focus disabled:text-content-disabled flex min-h-11 w-full items-center gap-2 rounded-md px-4 py-2 text-left text-base disabled:opacity-50 sm:text-sm"
      >
        <span
          id={valueId}
          className={clsx('min-w-0 flex-1 truncate', !selected && 'text-field-placeholder')}
        >
          {selected?.label ?? placeholder ?? ''}
        </span>
        {/* The well alone is not a signal on touch: `hover:` never matches on a phone, so without
            the chevron a Listbox and a TextField are the same rectangle and the user has to guess
            whether tapping opens a picker or a keyboard. */}
        <ChevronDown aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
      </button>
      {open && (
        <div
          ref={panelRef}
          id={panelId}
          role="listbox"
          aria-label={ariaLabel}
          aria-labelledby={labelledBy}
          onKeyDown={onPanelKeyDown}
          // An overlay with its own bounds is §4.4's dropdown/sheet exception, so this one nested
          // scroller is allowed: a forty-model catalog otherwise pushes the page past the field it
          // belongs to. Its ceiling is the measured one set above, not a token — see that effect.
          className={clsx(
            'bg-surface-highest absolute right-0 left-0 z-30 overflow-y-auto overscroll-contain rounded-lg p-1 shadow-lg select-none',
            drop === 'up' ? 'bottom-full mb-1' : 'top-full mt-1',
          )}
        >
          {options.map((option) => {
            const current = option.value === value
            return (
              <button
                key={String(option.value)}
                type="button"
                role="option"
                aria-selected={current}
                aria-disabled={option.disabled || undefined}
                tabIndex={-1}
                onClick={() => select(option)}
                className={clsx(
                  'flex min-h-11 w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition-colors',
                  option.disabled
                    ? 'text-content-disabled'
                    : clsx(
                        'hover:bg-row-bg-hover active:bg-row-bg-active',
                        current ? 'text-content-primary font-medium' : 'text-content-secondary',
                      ),
                )}
              >
                {/* Wraps rather than truncating: an option's label carries the badges and the
                    reason a model cannot be chosen, and the open panel is the one place there is
                    room to read them (§7). */}
                <span className="min-w-0 flex-1 break-words">{option.label}</span>
                {/* The unchecked check keeps its box so labels align and the panel width is
                    stable as the value moves. */}
                <Check
                  aria-hidden="true"
                  className={clsx('size-4 shrink-0', !current && 'opacity-0')}
                />
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
