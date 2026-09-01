import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { twMerge } from 'tailwind-merge'

/** Sheets stack: a confirmation opened from inside a sheet sits on top of it. Escape must dismiss
 *  ONE overlay — the topmost — so every open sheet registers here and only the last one acts. */
const openSheets: object[] = []

/** Everything Tab can actually reach. `[tabindex="-1"]` is deliberately excluded: a listbox's
 *  options are programmatic focus targets, and counting one as the panel's last control lets Tab
 *  escape the modal entirely. */
const focusableSelector =
  'a[href], summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])'

interface SheetProps {
  open: boolean
  /** Accessible name, when no visible heading carries it. */
  label?: string
  /** id of the visible heading `header` renders, when there is one. */
  labelledBy?: string
  /** Pinned above the scrolling body — a heading, a close control. */
  header?: ReactNode
  /** The body. It is the ONE thing inside the sheet that scrolls. */
  children: ReactNode
  bodyClassName?: string
  /** Pinned below the scrolling body, so a committing action never scrolls out of reach. */
  footer?: ReactNode
  onClose: () => void
}

/** The overlay for ARBITRARY content, and the mechanics every overlay in the app owes
 *  (design-language §7): portalled, scrim-dismissed, focus-trapped, focus-returned, `Escape`
 *  closes, and the body scroll is locked while it is open.
 *
 *  On a phone this is a BOTTOM SHEET, not a centred dialog nudged downwards: the switch is at
 *  `md:`, not `sm:`, because §1.5 makes 768px the SHAPE breakpoint — a 640px landscape phone still
 *  has a coarse pointer and a keyboard covering 40% of the screen. Full-bleed to the bottom edge,
 *  rounded on the free side only, safe-area padded, with its own body as the one thing that
 *  scrolls so a pinned footer stays reachable.
 *
 *  `Dialog` is this primitive with a confirm/cancel shape fixed on top of it. */
export function Sheet({
  open,
  label,
  labelledBy,
  header,
  children,
  bodyClassName,
  footer,
  onClose,
}: SheetProps) {
  const panel = useRef<HTMLDivElement>(null)
  const returnFocus = useRef<HTMLElement | null>(null)
  const identity = useRef({})
  // TWO effects, deliberately. The focus and scroll setup depends on `open` ALONE: callers pass
  // an inline `onClose`, so a new identity arrives on every render of the sheet's parent, and
  // with `onClose` in these deps the effect re-ran on each of them and called `panel.focus()` —
  // stealing focus out of any field inside the sheet after one keystroke. Re-registering the key
  // listener below is harmless by comparison, so it keeps `onClose` in its own deps.
  useEffect(() => {
    if (!open) return
    const self = identity.current
    openSheets.push(self)
    returnFocus.current = document.activeElement as HTMLElement | null
    panel.current?.focus()

    // Touch scrolling is not a Tab key: without this, dragging anywhere on the scrim scrolls the
    // page underneath the sheet, and the user lands somewhere else when it closes (§7).
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      const index = openSheets.lastIndexOf(self)
      if (index >= 0) openSheets.splice(index, 1)
      document.body.style.overflow = previousOverflow
      returnFocus.current?.focus()
    }
  }, [open])
  useEffect(() => {
    if (!open) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        if (openSheets.at(-1) === identity.current) onClose()
        return
      }
      if (event.key !== 'Tab' || !panel.current) return
      const controls = [...panel.current.querySelectorAll<HTMLElement>(focusableSelector)]
      if (controls.length === 0) return
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (document.activeElement === panel.current) {
        event.preventDefault()
        ;(event.shiftKey ? last : first).focus()
        return
      }
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      }
      if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }
    document.addEventListener('keydown', onKeyDown)
    return () => document.removeEventListener('keydown', onKeyDown)
  }, [onClose, open])
  if (!open) return null
  return createPortal(
    <div
      className="bg-media-scrim-bg/60 fixed inset-0 z-50 flex items-end justify-center md:items-center md:p-4"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose()
      }}
    >
      <div
        ref={panel}
        role="dialog"
        aria-modal="true"
        aria-label={labelledBy ? undefined : label}
        aria-labelledby={labelledBy}
        tabIndex={-1}
        // `max-h-sheet` in token terms: dvh tracks the mobile URL bar where vh does not. The
        // body is the only scroller inside the sheet, so a pinned footer stays put.
        className="bg-surface-highest max-h-sheet pb-safe-b flex w-full flex-col rounded-t-xl p-5 shadow-lg md:max-w-md md:rounded-xl md:p-6 md:pb-6"
      >
        {header}
        <div
          className={twMerge('min-h-0 flex-1 overflow-y-auto overscroll-contain', bodyClassName)}
        >
          {children}
        </div>
        {footer}
      </div>
    </div>,
    document.body,
  )
}
