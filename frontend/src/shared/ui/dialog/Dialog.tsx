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
      returnFocus.current?.focus()
    }
  }, [onClose, open])
  if (!open) return null
  return createPortal(
    <div
      className="bg-media-scrim-bg/60 fixed inset-0 z-50 flex items-end justify-center p-4 sm:items-center"
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
        className="bg-surface-highest w-full max-w-md rounded-xl p-6 shadow-lg"
      >
        <h2 id="dialog-title" className="text-lg font-semibold tracking-tight">
          {title}
        </h2>
        <div className="text-content-secondary mt-3 text-sm leading-relaxed">{children}</div>
        <div className="mt-6 flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose} disabled={pending}>
            취소
          </Button>
          <Button variant="cta" onClick={onConfirm} disabled={pending}>
            {pending ? '적용 중…' : confirmLabel}
          </Button>
        </div>
      </div>
    </div>,
    document.body,
  )
}
