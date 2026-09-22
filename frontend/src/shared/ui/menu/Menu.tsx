import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ComponentType,
  type KeyboardEvent,
  type ReactNode,
} from 'react'
import { clsx } from 'clsx'
import { Check, ChevronDown } from 'lucide-react'
import { Button } from '../button/Button'
import { MENU_VIEWPORT_GUTTER_PX } from './config'

export interface MenuOption<T extends string> {
  value: T
  label: string
  /** Drawn before the label when a choice has a glyph of its own. It belongs to the OPTION, not
   *  to the trigger: a trigger that changes glyph with the stored value has to stand for every
   *  choice at once, which is what makes it unreadable, while a row states one choice only. Give
   *  it to every option of a menu or to none, so the labels stay on one column. */
  icon?: ComponentType<{ className?: string }>
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
  triggerLabel,
  triggerDescription,
  value,
  options,
  onChange,
  triggerIcon,
  triggerClassName,
}: {
  /** Accessible name of the menu, and of the trigger while that trigger is icon-only. */
  label: string
  /** Makes the trigger WEAR the current choice instead of an icon alone: `triggerIcon`, this
   *  text and a chevron, on no plane at rest. The visible text then IS the trigger's accessible
   *  name (WCAG 2.5.3), so `label` names the open panel alone. The panel is centred under the
   *  trigger and constrained to the viewport, including when translated labels wrap.
   *  Used by the phone's group row, where the name of the destination the address is under is
   *  itself what opens the group (THEME-38). */
  triggerLabel?: string
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
  const [panelMaxHeight, setPanelMaxHeight] = useState<number>()

  // The group name can sit below a tall, enlarged header. Measure the remaining room so every
  // destination stays reachable inside the menu, even before that header has scrolled away.
  useLayoutEffect(() => {
    if (!open || triggerLabel === undefined) return
    const measure = () => {
      const panel = panelRef.current
      if (panel)
        setPanelMaxHeight(
          Math.max(
            0,
            window.innerHeight - panel.getBoundingClientRect().top - MENU_VIEWPORT_GUTTER_PX,
          ),
        )
    }
    measure()
    window.addEventListener('resize', measure)
    window.addEventListener('scroll', measure, true)
    return () => {
      window.removeEventListener('resize', measure)
      window.removeEventListener('scroll', measure, true)
    }
  }, [open, triggerLabel])

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
    <div ref={rootRef} className="relative inline-flex max-w-full min-w-0">
      <Button
        ref={triggerRef}
        variant={triggerLabel === undefined ? 'secondary' : 'ghost'}
        size={triggerLabel === undefined ? 'icon' : 'default'}
        aria-label={triggerLabel === undefined ? label : undefined}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        aria-describedby={triggerDescription ? descriptionId : undefined}
        className={clsx('max-w-full', triggerClassName)}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onTriggerKeyDown}
      >
        {triggerIcon}
        {triggerLabel !== undefined && (
          <>
            <span className="min-w-0 truncate">{triggerLabel}</span>
            {/* The one thing that says the name is pressable: it turns over with the panel. */}
            <ChevronDown
              aria-hidden="true"
              className={clsx(
                'duration-fast ease-standard size-4 shrink-0 transition-transform',
                open && 'rotate-180',
              )}
            />
          </>
        )}
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
          style={triggerLabel === undefined ? undefined : { maxHeight: panelMaxHeight }}
          className={clsx(
            'bg-surface-highest absolute top-full z-30 mt-2 rounded-lg p-1 shadow-lg select-none',
            triggerLabel === undefined
              ? 'right-0 min-w-44'
              : 'left-1/2 w-max max-w-[calc(100vw-2rem)] -translate-x-1/2 overflow-y-auto overscroll-contain',
          )}
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
                  'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-9 w-full items-center gap-3 rounded-md px-3 text-sm transition-colors pointer-coarse:min-h-11',
                  triggerLabel === undefined ? 'whitespace-nowrap' : 'whitespace-normal',
                  checked ? 'text-content-primary font-medium' : 'text-content-secondary',
                )}
              >
                {option.icon && <option.icon aria-hidden="true" className="size-4 shrink-0" />}
                <span className="min-w-0 flex-1 text-left break-words">{option.label}</span>
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
