import { useId, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowRight, X } from 'lucide-react'
import { Button, Sheet, Typography, buttonStyles } from '@/shared/ui'
import { KIT_HASH, KIT_HREF, type KitDefaultKey } from '../model/kit'
import { KitRing } from './KitRing'
import { KitStatus } from './KitStatus'
import { useKitElement } from './useKitElement'
import { useKitText } from './useKitText'

/** Guide: a kit piece's account default, changed on 홈 without leaving it. The sheet opens on
 *  the piece as the picture draws it — its ring, its name, its state and its one fact, read from
 *  the saved data so they move the moment a change is saved — then one line on what the default
 *  does, the control that changes it (the caller's: the same one the piece's own screen uses),
 *  and at its foot the way to that full screen.
 *
 *  One frame for every piece that has a default, so 말투, 지침 and 글 쓰는 AI open the same way. */
export function KitPieceSheet({
  kit,
  open,
  onClose,
  size,
  children,
}: {
  kit: KitDefaultKey
  open: boolean
  onClose: () => void
  /** `wide` for a control that is a whole list of its own (지침). */
  size?: 'default' | 'wide'
  children: ReactNode
}) {
  const { t } = useTranslation(['creation', 'common'])
  const text = useKitText()
  const element = useKitElement(kit)
  const headingId = useId()
  return (
    <Sheet
      open={open}
      labelledBy={headingId}
      onClose={onClose}
      size={size}
      header={
        <div className="mb-4 flex items-start gap-3">
          <KitRing kitKey={kit} state={element.state} fill={element.fill} size="sm" />
          <div className="flex min-w-0 flex-1 flex-col">
            <Typography variant="title" as="h2" id={headingId} className="break-words">
              {text.name(element)}
            </Typography>
            <KitStatus kit={kit} />
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('action.close', { ns: 'common' })}
            onClick={onClose}
          >
            <X aria-hidden="true" className="size-5" />
          </Button>
        </div>
      }
      footer={
        <Link
          to={KIT_HREF[kit]}
          hash={KIT_HASH[kit]}
          className={buttonStyles({ variant: 'secondary', className: 'mt-4 w-full' })}
        >
          {t(`kit.sheet.open.${kit}`)}
          <ArrowRight aria-hidden="true" className="size-4" />
        </Link>
      }
    >
      <Typography variant="body" className="text-content-secondary text-pretty">
        {t(`kit.sheet.help.${kit}`)}
      </Typography>
      <div className="mt-4">{children}</div>
    </Sheet>
  )
}
