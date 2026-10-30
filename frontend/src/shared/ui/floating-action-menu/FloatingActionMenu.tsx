import { useEffect, useId, useRef, useState, type ComponentType, type KeyboardEvent } from 'react'
import { clsx } from 'clsx'
import { Plus, X } from 'lucide-react'
import { ActionBar } from '../action-bar/ActionBar'
import { Button } from '../button/Button'
import { useNavigationContext, type NavigationLinkProps } from '../navigation/context'

export interface FloatingActionItem {
  id: string
  label: string
  icon: ComponentType<{ className?: string }>
  href: string
  /** Runs before the navigation, e.g. to record where a creation was started from. */
  onSelect?: () => void
}

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}

/** One button that opens a stacked menu above itself (THEME-67) — a channel's every other action
 *  one thumb-reach tap away while its list keeps the screen.
 *
 *  It sits in the list add-action dock (THEME-24): lower band, right-aligned at every width, the
 *  control floating on its own shadow with no plane. Opening dims the page under a scrim — the
 *  dock rises above the sticky header so the scrim covers it too — and the same button turns into
 *  the way out (×) in place. The groups stand above it in the order given, the last one nearest
 *  the thumb, so the primary action goes last.
 *
 *  The items are ADDRESSES, so they are links: a new tab or a copied link works. Keys follow the
 *  WAI-APG menu button as `ActionMenu` does — the trigger is the one tab stop, the rows take
 *  programmatic focus, ↑ ↓ cross the groups, Home/End jump, Escape closes and returns focus. */
export function FloatingActionMenu({
  label,
  closeLabel,
  menuLabel,
  groups,
  className,
}: {
  /** The closed button's visible text, after its plus glyph. */
  label: string
  /** The open button's accessible name; it shows × alone. */
  closeLabel: string
  menuLabel: string
  groups: readonly (readonly FloatingActionItem[])[]
  className?: string
}) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const Link = useNavigationContext()?.Link ?? NativeLink

  const rows = () =>
    Array.from(listRef.current?.querySelectorAll<HTMLElement>('[role="menuitem"]') ?? [])

  const close = (returnFocus: boolean) => {
    setOpen(false)
    if (returnFocus) queueMicrotask(() => triggerRef.current?.focus())
  }

  useEffect(() => {
    if (!open) return
    queueMicrotask(() => rows()[0]?.focus())
    // Escape works wherever focus is while the menu is open, the × button included.
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      close(true)
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [open])

  const onListKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Tab') {
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
    const current = elements.indexOf(document.activeElement as HTMLElement)
    elements[(current + step + elements.length) % elements.length]?.focus()
  }

  return (
    <ActionBar dock="list" className={clsx(open && 'z-40', className)}>
      {open && (
        <div
          aria-hidden="true"
          data-testid="floating-action-scrim"
          onClick={() => close(true)}
          className="bg-media-scrim-bg/60 motion-safe:animate-scrim-in fixed inset-0"
        />
      )}
      {/* The dock gives each direct child the floating shadow; this wrapper takes it in the
          button's own shape, and the menu hangs off it without widening it. */}
      <div className="relative rounded-full">
        {open && (
          <div
            ref={listRef}
            id={id}
            role="menu"
            aria-label={menuLabel}
            onKeyDown={onListKeyDown}
            className="absolute right-0 bottom-full mb-3 flex w-max max-w-[calc(100vw-2rem)] flex-col items-end gap-2"
          >
            {groups.map((items, index) => (
              <div
                key={index}
                role="group"
                className="bg-surface-highest motion-safe:animate-rise flex min-w-44 flex-col rounded-lg p-1 shadow-lg"
              >
                {items.map((item) => (
                  <Link
                    key={item.id}
                    href={item.href}
                    role="menuitem"
                    tabIndex={-1}
                    onClick={() => {
                      item.onSelect?.()
                      close(false)
                    }}
                    className="text-content-primary hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center gap-3 rounded-md px-3 text-sm font-medium"
                  >
                    <item.icon aria-hidden="true" className="size-5 shrink-0" />
                    {item.label}
                  </Link>
                ))}
              </div>
            ))}
          </div>
        )}
        <Button
          ref={triggerRef}
          variant={open ? 'secondary' : 'cta'}
          size={open ? 'icon' : 'default'}
          aria-label={open ? closeLabel : undefined}
          aria-haspopup="menu"
          aria-expanded={open}
          aria-controls={open ? id : undefined}
          onClick={() => (open ? close(true) : setOpen(true))}
          onKeyDown={(event) => {
            if (open || (event.key !== 'ArrowDown' && event.key !== 'ArrowUp')) return
            event.preventDefault()
            setOpen(true)
          }}
          className="rounded-full"
        >
          {open ? (
            <X aria-hidden="true" className="size-5" />
          ) : (
            <>
              <Plus aria-hidden="true" className="size-5 shrink-0" />
              {label}
            </>
          )}
        </Button>
      </div>
    </ActionBar>
  )
}
