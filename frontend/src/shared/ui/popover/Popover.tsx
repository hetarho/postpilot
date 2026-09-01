import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../button/Button'

const focusableSelector =
  'a[href], summary, input:not([disabled]), select:not([disabled]), textarea:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"]):not([disabled])'

export function Popover({
  label,
  triggerLabel,
  triggerClassName,
  children,
  disabled = false,
  placement = 'above',
}: {
  label: string
  triggerLabel?: ReactNode
  triggerClassName?: string
  children: (close: () => void) => ReactNode
  disabled?: boolean
  /** Above remains the action-bar default; compact header controls explicitly open below. */
  placement?: 'above' | 'below'
}) {
  const { t } = useTranslation('common')
  const [open, setOpen] = useState(false)
  const id = useId()
  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const panelRef = useRef<HTMLDivElement>(null)

  const close = () => {
    setOpen(false)
  }

  useEffect(() => {
    if (!open) return
    const focusableElements = () =>
      Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? [])
    queueMicrotask(() => {
      const firstElement = focusableElements()[0]
      if (firstElement) firstElement.focus()
      else panelRef.current?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) close()
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
  }, [open])

  return (
    <div ref={rootRef} className="relative inline-flex">
      <Button
        ref={triggerRef}
        variant="secondary"
        aria-label={label}
        aria-haspopup="dialog"
        aria-expanded={open}
        aria-controls={open ? id : undefined}
        disabled={disabled}
        className={triggerClassName}
        onClick={() => setOpen((value) => !value)}
      >
        {triggerLabel ?? t('popover.options')}
      </Button>
      {open && (
        <div
          ref={panelRef}
          id={id}
          role="dialog"
          tabIndex={-1}
          aria-label={label}
          className={`bg-surface-highest absolute right-0 z-30 w-72 rounded-lg p-4 shadow-lg ${
            placement === 'below'
              ? 'max-h-popover-below-max top-full mt-2 overflow-y-auto overscroll-contain'
              : 'bottom-full mb-2'
          }`}
        >
          {children(close)}
        </div>
      )}
    </div>
  )
}
