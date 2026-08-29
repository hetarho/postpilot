import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { Button } from '../button/Button'

export function Popover({
  label,
  children,
  disabled = false,
}: {
  label: string
  children: (close: () => void) => ReactNode
  disabled?: boolean
}) {
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
    queueMicrotask(() => {
      panelRef.current
        ?.querySelector<HTMLElement>(
          'input:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"])',
        )
        ?.focus()
    })
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) close()
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return
      event.preventDefault()
      close()
      queueMicrotask(() => triggerRef.current?.focus())
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
        onClick={() => setOpen((value) => !value)}
      >
        옵션
      </Button>
      {open && (
        <div
          ref={panelRef}
          id={id}
          role="dialog"
          aria-label={label}
          className="bg-surface-raised border-border-subtle absolute right-0 bottom-full z-30 mb-2 w-[min(20rem,calc(100vw-2rem))] rounded-lg border p-4 shadow-lg"
        >
          {children(close)}
        </div>
      )}
    </div>
  )
}
