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
        /* ONE row on a phone, 3 : 7, collapsing to the desktop right-aligned row from `md:` up.
           The CTA is last (§4) and therefore on the right, which is the side a right-handed
           one-handed grip reaches first, and it is the wider of the two because confirming is
           what the sheet was opened to do — 취소 needs only its two syllables. Stacking them
           full-width, which this replaced, spent two 44px rows plus a gap on a decision with one
           obvious answer, on the shape that has the least room for it. */
        <div className="mt-6 grid grid-cols-[3fr_7fr] gap-2 md:flex md:justify-end">
          <Button variant="ghost" onClick={onClose} disabled={pending}>
            {t('dialog.cancel')}
          </Button>
          <Button variant="cta" onClick={onConfirm} pending={pending}>
            {confirmLabel}
          </Button>
        </div>
      }
    >
      {children}
    </Sheet>
  )
}
