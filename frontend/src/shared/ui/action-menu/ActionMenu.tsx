import { useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ChevronDown, X } from 'lucide-react'
import { Button } from '../button/Button'
import type { ButtonVariant } from '../button/buttonStyles'
import { Sheet } from '../sheet/Sheet'
import { SM_MEDIA_QUERY, useMediaQuery } from '../media-query/useMediaQuery'

export interface ActionMenuItem {
  id: string
  label: string
  /** A second line saying what the action does. */
  description?: string
  /** Why the action cannot run now. Present, the row stays in the menu — so the reason can be
   *  read where the action is looked for — and selecting it does nothing. */
  disabledReason?: string
  onSelect: () => void
}

/** A trigger that opens a short list of ACTIONS (WAI-APG menu button). It is not `Menu`: that
 *  one picks one value (`menuitemradio`), this one runs a command (`menuitem`), and a disabled
 *  action keeps its row with the reason it cannot run.
 *
 *  The trigger stays the only tab stop; the rows take programmatic focus, ↑ ↓ move, Home/End
 *  jump, Enter runs, Escape closes and returns focus to the trigger. Below `sm:` the list is a
 *  bottom sheet with a visible way out, as `Popover phone="sheet"` is (THEME-29). */
export function ActionMenu({
  label,
  items,
  triggerLabel,
  triggerVariant = 'secondary',
  disabled = false,
  className,
}: {
  /** The trigger's accessible name, and the open list's. */
  label: string
  items: readonly ActionMenuItem[]
  /** Visible trigger content; a ▾ glyph alone when absent. */
  triggerLabel?: ReactNode
  triggerVariant?: ButtonVariant
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('common')
  const [open, setOpen] = useState(false)
  const id = useId()
  const headingId = `${id}-heading`
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const wide = useMediaQuery(SM_MEDIA_QUERY)

  const rows = () =>
    Array.from(listRef.current?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? [])

  const close = (returnFocus: boolean) => {
    setOpen(false)
    if (returnFocus) queueMicrotask(() => triggerRef.current?.focus())
  }

  useEffect(() => {
    if (!open) return
    queueMicrotask(() => rows()[0]?.focus())
    if (!wide) return
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) close(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    return () => document.removeEventListener('pointerdown', onPointerDown)
  }, [open, wide])

  const onListKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      event.stopPropagation()
      close(true)
      return
    }
    if (event.key === 'Tab' && wide) {
      // The rows are programmatic stops, so Tab continues from the trigger; keep the row mounted
      // until the native traversal completes.
      setTimeout(() => setOpen(false), 0)
      return
    }
    const elements = rows()
    if (!elements.length) return
    if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault()
      elements.at(event.key === 'Home' ? 0 : -1)?.focus()
      return
    }
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    const step = event.key === 'ArrowDown' ? 1 : -1
    const current = elements.indexOf(document.activeElement as HTMLButtonElement)
    elements[(current + step + elements.length) % elements.length]?.focus()
  }

  const onTriggerKeyDown = (event: KeyboardEvent<HTMLButtonElement>) => {
    if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
    event.preventDefault()
    setOpen(true)
  }

  const list = (
    <div
      ref={listRef}
      id={id}
      role="menu"
      aria-label={label}
      onKeyDown={onListKeyDown}
      className={clsx(
        'select-none',
        wide &&
          'bg-surface-highest absolute right-0 bottom-full z-30 mb-2 min-w-56 rounded-lg p-1 shadow-lg',
      )}
    >
      {items.map((item) => {
        const blocked = Boolean(item.disabledReason)
        const note = item.disabledReason ?? item.description
        const noteId = note ? `${id}-${item.id}-note` : undefined
        const labelId = `${id}-${item.id}-label`
        return (
          <button
            key={item.id}
            type="button"
            role="menuitem"
            tabIndex={-1}
            aria-disabled={blocked || undefined}
            // Named by its label alone; the second line — what it does, or why it cannot run —
            // is its description.
            aria-labelledby={labelId}
            aria-describedby={noteId}
            onClick={() => {
              if (blocked) return
              close(true)
              item.onSelect()
            }}
            className={clsx(
              'flex min-h-9 w-full flex-col items-start justify-center rounded-md px-3 py-2 text-left text-sm transition-colors pointer-coarse:min-h-11',
              blocked
                ? 'text-content-tertiary cursor-not-allowed'
                : 'text-content-primary hover:bg-row-bg-hover active:bg-row-bg-active',
            )}
          >
            <span id={labelId} className="font-medium">
              {item.label}
            </span>
            {note && (
              <span id={noteId} className="text-content-secondary mt-0.5 text-xs">
                {note}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )

  return (
    <div ref={rootRef} className={clsx('relative inline-flex', className)}>
      <Button
        ref={triggerRef}
        variant={triggerVariant}
        size={triggerLabel === undefined ? 'icon' : 'default'}
        aria-label={label}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open && wide ? id : undefined}
        disabled={disabled}
        onClick={() => setOpen((current) => !current)}
        onKeyDown={onTriggerKeyDown}
      >
        {triggerLabel}
        <ChevronDown
          aria-hidden="true"
          className={clsx(
            'duration-fast ease-standard size-4 shrink-0 transition-transform',
            open && 'rotate-180',
          )}
        />
      </Button>
      {wide ? (
        open && list
      ) : (
        <Sheet
          open={open}
          labelledBy={headingId}
          onClose={() => close(true)}
          header={
            <div className="mb-3 flex items-center justify-between gap-3">
              <h2 id={headingId} className="min-w-0 truncate text-lg font-semibold tracking-tight">
                {label}
              </h2>
              <Button
                variant="ghost"
                size="icon"
                aria-label={t('action.close')}
                onClick={() => close(true)}
              >
                <X aria-hidden="true" className="size-5" />
              </Button>
            </div>
          }
        >
          {list}
        </Sheet>
      )}
    </div>
  )
}
