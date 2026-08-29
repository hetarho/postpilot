import { useEffect, useRef, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Button } from '../button/Button'

interface DialogProps {
  open: boolean
  title: string
  children: ReactNode
  confirmLabel: string
  onConfirm: () => void
  onClose: () => void
  pending?: boolean
}

/** On a phone this is a BOTTOM SHEET, not a centred dialog nudged downwards (design-language §7):
 *  the switch is at `md:`, not `sm:`, because §1.5 makes 768px the SHAPE breakpoint — a 640px
 *  landscape phone still has a coarse pointer and a keyboard covering 40% of the screen.
 *  full-bleed to the bottom edge, rounded on the free side only, safe-area padded, with its own
 *  body as the one thing that scrolls. It becomes a centred dialog from `sm:` up. */
export function Dialog({
  open,
  title,
  children,
  confirmLabel,
  onConfirm,
  onClose,
  pending = false,
}: DialogProps) {
  const panel = useRef<HTMLDivElement>(null)
  const returnFocus = useRef<HTMLElement | null>(null)
  useEffect(() => {
    if (!open) return
    returnFocus.current = document.activeElement as HTMLElement | null
    panel.current?.focus()

    // Touch scrolling is not a Tab key: without this, dragging anywhere on the scrim scrolls the
    // page underneath the sheet, and the user lands somewhere else when it closes (§7).
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
      if (event.key !== 'Tab' || !panel.current) return
      const controls = [
        ...panel.current.querySelectorAll<HTMLElement>(
          'button,[href],input,select,textarea,[tabindex]:not([tabindex="-1"])',
        ),
      ]
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
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = previousOverflow
      returnFocus.current?.focus()
    }
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
        aria-labelledby="dialog-title"
        tabIndex={-1}
        // `max-h-sheet` in token terms: dvh tracks the mobile URL bar where vh does not. The
        // body is the only scroller inside the sheet, so the confirm row stays pinned.
        className="bg-surface-highest max-h-sheet pb-safe-b flex w-full flex-col rounded-t-xl p-5 shadow-lg md:max-w-md md:rounded-xl md:p-6 md:pb-6"
      >
        <h2 id="dialog-title" className="text-lg font-semibold tracking-tight">
          {title}
        </h2>
        <div className="text-content-secondary mt-3 min-h-0 flex-1 overflow-y-auto overscroll-contain text-sm leading-relaxed">
          {children}
        </div>
        {/* Full-width stacked targets on a phone — the §4.2/§4.3 shape for a committing pair —
            collapsing to the desktop right-aligned row from `sm:` up. The CTA is last (§4). */}
        <div className="mt-6 grid gap-2 pb-5 md:flex md:justify-end md:pb-0">
          <Button variant="ghost" onClick={onClose} disabled={pending} className="md:order-1">
            취소
          </Button>
          <Button variant="cta" onClick={onConfirm} pending={pending} className="md:order-2">
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
