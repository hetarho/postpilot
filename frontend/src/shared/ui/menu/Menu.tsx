import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import { Button } from '../button/Button'

export interface MenuOption<T extends string> {
  value: T
  label: string
}

/** The app-drawn dropdown for one bounded choice (design-language §7). A native select's open
 *  option list is OS-drawn, so it cannot wear the app's surfaces — this menu exists so a compact
 *  trigger can offer a short option list without breaking the design system the moment it opens.
 *
 *  WAI-APG menu-button semantics: the trigger is a `Button` with `aria-haspopup="menu"`, options
 *  are `menuitemradio` rows with roving programmatic focus (all `tabIndex={-1}`; the trigger stays
 *  the only tab stop). Tab closes and continues to the adjacent page control; Escape closes and
 *  returns to the trigger. */
export function Menu<T extends string>({
  label,
  triggerDescription,
  value,
  options,
  onChange,
  triggerIcon,
  triggerClassName,
}: {
  /** Accessible name of both the trigger and the menu — the trigger itself is icon-only. */
  label: string
  /** Optional state announced with the closed trigger without bloating its concise name. */
  triggerDescription?: string
  value: T
  options: readonly MenuOption<T>[]
  onChange: (value: T) => void
  triggerIcon: ReactNode
  triggerClassName?: string
}) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const descriptionId = `${id}-description`
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  const items = () =>
    Array.from(
      panelRef.current?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]') ?? [],
    )

  const close = (returnFocus: boolean) => {
    setOpen(false)
    if (returnFocus) queueMicrotask(() => triggerRef.current?.focus())
  }

  const select = (next: T) => {
    close(true)
    if (next !== value) onChange(next)
  }

  useEffect(() => {
    if (!open) return
    // The checked option receives focus so the keyboard arrives where the state already is —
    // read off the rendered rows so the effect depends on nothing but being open.
    queueMicrotask(() => {
      const elements = items()
      const checked = elements.find((element) => element.getAttribute('aria-checked') === 'true')
      ;(checked ?? elements[0])?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) close(true)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open])

  const onMenuKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      close(true)
      return
    }
    if (event.key === 'Tab') {
      // Do not prevent the native traversal: menu items are programmatic stops (`tabIndex=-1`),
      // so forward Tab reaches the next page control and Shift+Tab reaches the trigger. Keep the
      // focused row mounted until that default traversal completes; unmounting it synchronously
      // makes the browser restart from the trigger and costs an extra key press.
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
    <div ref={rootRef} className="relative inline-flex">
      <Button
        ref={triggerRef}
        variant="secondary"
        size="icon"
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        aria-describedby={triggerDescription ? descriptionId : undefined}
        className={triggerClassName}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onTriggerKeyDown}
      >
        {triggerIcon}
      </Button>
      {triggerDescription && (
        <span id={descriptionId} className="sr-only">
          {triggerDescription}
        </span>
      )}
      {open && (
        <div
          ref={panelRef}
          id={id}
          role="menu"
          aria-label={label}
          onKeyDown={onMenuKeyDown}
          className="bg-surface-highest absolute top-full right-0 z-30 mt-2 min-w-44 rounded-lg p-1 shadow-lg select-none"
        >
          {options.map((option) => {
            const checked = option.value === value
            return (
              <button
                key={option.value}
                type="button"
                role="menuitemradio"
                aria-checked={checked}
                tabIndex={-1}
                onClick={() => select(option.value)}
                className={clsx(
                  'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 w-full items-center gap-3 rounded-md px-3 text-sm whitespace-nowrap transition-colors',
                  checked ? 'text-content-primary font-medium' : 'text-content-secondary',
                )}
              >
                <span className="min-w-0 flex-1 text-left">{option.label}</span>
                {/* The unchecked check keeps its box so labels align and the panel width is stable. */}
                <Check
                  aria-hidden="true"
                  className={clsx('size-4 shrink-0', !checked && 'opacity-0')}
                />
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
