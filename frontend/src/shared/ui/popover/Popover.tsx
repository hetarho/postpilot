import { useEffect, useId, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { twMerge } from 'tailwind-merge'
import { POPOVER_VIEWPORT_GUTTER_PX } from '@/shared/config'
import { Button } from '../button/Button'
import { Sheet } from '../sheet/Sheet'
import { SM_MEDIA_QUERY, useMediaQuery } from './useMediaQuery'

const focusableSelector =
  'a[href], summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])'

export function Popover({
  label,
  triggerLabel,
  triggerClassName,
  children,
  disabled = false,
  placement = 'above',
  align = 'end',
  phone = 'popover',
  className,
}: {
  label: string
  triggerLabel?: ReactNode
  triggerClassName?: string
  children: (close: () => void) => ReactNode
  disabled?: boolean
  /** Above remains the action-bar default; compact header controls explicitly open below. */
  placement?: 'above' | 'below'
  /** Which edge of the panel is pinned to the trigger. `end` is the action-bar default: a dock
   *  control's panel grows leftwards, away from the viewport edge it sits nearest. */
  align?: 'start' | 'end'
  /** `sheet` swaps the panel for a full-bleed bottom sheet below `sm:`. A 288px panel opening
   *  over the draft is a desktop shape borrowed by a phone; a long brief needs the whole screen
   *  and its own scroller (design-language §7). */
  phone?: 'popover' | 'sheet'
  /** On the popover's ROOT, so a caller can make the trigger share a flex row. */
  className?: string
}) {
  const { t } = useTranslation('common')
  const [open, setOpen] = useState(false)
  const id = useId()
  const headingId = `${id}-heading`
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)
  const wide = useMediaQuery(SM_MEDIA_QUERY)
  // The two shapes cannot both be mounted: an open `Sheet` locks the body scroll and traps focus
  // regardless of whether CSS is painting it.
  const asSheet = phone === 'sheet' && !wide

  const close = () => {
    setOpen(false)
  }

  // The panel is anchored to the trigger, and a trigger can sit anywhere along the width. Nothing
  // in CSS knows where that puts a fixed-width panel, so the overflow is measured and corrected
  // here — inside the viewport gutters, on whichever side it ran past.
  useLayoutEffect(() => {
    const panel = panelRef.current
    if (!open || asSheet || !panel) return
    panel.style.transform = ''
    const rect = panel.getBoundingClientRect()
    if (!rect.width) return
    let shift = 0
    const overflowRight = rect.right - (window.innerWidth - POPOVER_VIEWPORT_GUTTER_PX)
    if (overflowRight > 0) shift = -overflowRight
    const overflowLeft = POPOVER_VIEWPORT_GUTTER_PX - (rect.left + shift)
    if (overflowLeft > 0) shift += overflowLeft
    if (shift) panel.style.transform = `translateX(${shift}px)`
  }, [asSheet, open])

  useEffect(() => {
    if (!open || asSheet) return
    const focusableElements = () =>
      Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])
    queueMicrotask(() => {
      const firstElement = focusableElements()[0]
      if (firstElement) firstElement.focus()
      else panelRef.current?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      const target = event.target as Element | null
      // A press inside a MODAL opened from this panel is not a press outside it: the dialog is
      // portalled to the body, so closing here would unmount the control that opened it and take
      // the dialog down with it mid-confirmation.
      if (target?.closest?.('[aria-modal="true"]')) return
      if (!rootRef.current?.contains(target)) close()
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        close()
        queueMicrotask(() => triggerRef.current?.focus())
        return
      }
      if (event.key !== 'Tab') return

      const elements = focusableElements()
      const firstElement = elements[0]
      const lastElement = elements.at(-1)
      const activeElement = document.activeElement
      if (!firstElement || !lastElement) {
        event.preventDefault()
        panelRef.current?.focus()
        return
      }
      if (
        event.shiftKey &&
        (activeElement === firstElement || !panelRef.current?.contains(activeElement))
      ) {
        event.preventDefault()
        lastElement.focus()
      } else if (
        !event.shiftKey &&
        (activeElement === lastElement || !panelRef.current?.contains(activeElement))
      ) {
        event.preventDefault()
        firstElement.focus()
      }
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [asSheet, open])

  const trigger = (
    <Button
      ref={triggerRef}
      variant="secondary"
      aria-label={label}
      aria-haspopup="dialog"
      aria-expanded={open}
      // The sheet is `Sheet`'s own portalled panel and carries no id of ours, so pointing at one
      // there would be a dangling reference.
      aria-controls={open && !asSheet ? id : undefined}
      disabled={disabled}
      className={triggerClassName}
      onClick={() => setOpen((value) => !value)}
    >
      {triggerLabel ?? t('popover.options')}
    </Button>
  )

  if (asSheet) {
    return (
      <div ref={rootRef} className={twMerge('inline-flex', className)}>
        {trigger}
        <Sheet
          open={open}
          labelledBy={headingId}
          onClose={close}
          header={
            /* A visible way out, beside the name of what is open: on a phone the scrim is a
               narrow strip above a full-bleed sheet, and Escape is not a gesture a touch user
               has (§7). */
            <div className="mb-3 flex items-center justify-between gap-3">
              <h2 id={headingId} className="min-w-0 truncate text-lg font-semibold tracking-tight">
                {label}
              </h2>
              <Button variant="ghost" size="icon" aria-label={t('action.close')} onClick={close}>
                <X aria-hidden="true" className="size-5" />
              </Button>
            </div>
          }
        >
          {children(close)}
        </Sheet>
      </div>
    )
  }

  return (
    <div ref={rootRef} className={twMerge('relative inline-flex', className)}>
      {trigger}
      {open && (
        <div
          ref={panelRef}
          id={id}
          role="dialog"
          tabIndex={-1}
          aria-label={label}
          className={`bg-surface-highest max-w-popover absolute z-30 w-72 rounded-lg p-4 shadow-lg ${
            align === 'start' ? 'left-0' : 'right-0'
          } ${
            placement === 'below'
              ? 'max-h-popover-below-max top-full mt-2 overflow-y-auto overscroll-contain'
              : 'max-h-popover-above-max bottom-full mb-2 overflow-y-auto overscroll-contain'
          }`}
        >
          {children(close)}
        </div>
      )}
    </div>
  )
}
