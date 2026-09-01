import { type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../button/Button'
import { Sheet } from '../sheet/Sheet'

interface DialogProps {
  open: boolean
  title: string
  children: ReactNode
  confirmLabel: string
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
        /* Full-width stacked targets on a phone — the §4.2/§4.3 shape for a committing pair —
           collapsing to the desktop right-aligned row from `sm:` up. The CTA is last (§4). */
        <div className="mt-6 grid gap-2 pb-5 md:flex md:justify-end md:pb-0">
          <Button variant="ghost" onClick={onClose} disabled={pending} className="md:order-1">
            {t('dialog.cancel')}
          </Button>
          <Button variant="cta" onClick={onConfirm} pending={pending} className="md:order-2">
            {confirmLabel}
          </Button>
        </div>
      }
    >
      {children}
    </Sheet>
  )
}
