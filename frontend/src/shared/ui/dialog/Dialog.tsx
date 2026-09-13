import { type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../button/Button'
import { Sheet } from '../sheet/Sheet'

interface DialogProps {
  open: boolean
  title: string
  children: ReactNode
  confirmLabel: string
  cancelLabel?: string
  onConfirm: () => void
  onClose: () => void
  pending?: boolean
}

/** `Sheet` with the confirm shape fixed on top: one title, one explanation, cancel and confirm.
 *  A destructive or irreversible action is confirmed through this and never through
 *  `window.confirm`, which mobile browsers let the user suppress permanently (design-language §7).
 *  Everything about the overlay itself — the phone bottom sheet, the focus trap, the scroll lock —
 *  belongs to `Sheet`; reach for that one directly when the content is not a confirmation. */
export function Dialog({
  open,
  title,
  children,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onClose,
  pending = false,
}: DialogProps) {
  const { t } = useTranslation('common')
  return (
    <Sheet
      open={open}
      labelledBy="dialog-title"
      onClose={onClose}
      header={
        <h2 id="dialog-title" className="text-lg font-semibold tracking-tight">
          {title}
        </h2>
      }
      bodyClassName="text-content-secondary mt-3 text-sm leading-relaxed"
      footer={
        /* Keep the 3 : 7 phone row when its container has enough room. At increased
           text size, stack the actions before labels collapse to one letter per line.
           The committing action stays last; the desktop keeps its natural-width row. */
        <div className="@container">
          <div className="mt-6 grid grid-cols-1 gap-2 md:flex md:justify-end @3xs:grid-cols-[3fr_7fr]">
            <Button variant="ghost" onClick={onClose} disabled={pending}>
              {cancelLabel ?? t('dialog.cancel')}
            </Button>
            <Button variant="cta" onClick={onConfirm} pending={pending}>
              {confirmLabel}
            </Button>
          </div>
        </div>
      }
    >
      {children}
    </Sheet>
  )
}
