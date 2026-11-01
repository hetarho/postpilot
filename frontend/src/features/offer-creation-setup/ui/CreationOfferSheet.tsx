import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Sheet, Typography } from '@/shared/ui'
import type { CreationOfferKind, CreationOfferMissing } from '../model/offer'

interface CreationOfferSheetProps {
  kind: CreationOfferKind
  open: boolean
  missing: CreationOfferMissing | undefined
  onCreateVoice: () => void
  onContinueVoice: (voiceId: string) => void
  onCreateTemplate: () => void
  /** `이대로 쓰기` / `이대로 만들기`, Escape and the scrim: closed for this creation only. */
  onClose: () => void
  /** `다시 묻지 않기`. */
  onDismiss: () => void
}

/** The offer a new post or clip opens on when the account lacks what it is written with
 *  (POST-121, CLIP-223): a title naming what is missing, one line, and one action per missing
 *  kind. It forces nothing — closing it is as plain as any action — and it renders no sibling
 *  feature (ARCH-13): the page opens the voice sheet or routes to the wizard. */
export function CreationOfferSheet({
  kind,
  open,
  missing,
  onCreateVoice,
  onContinueVoice,
  onCreateTemplate,
  onClose,
  onDismiss,
}: CreationOfferSheetProps) {
  const { t } = useTranslation('creation')
  const titleId = useId()
  if (!missing) return null
  const voice = kind === 'post' ? missing.voice : undefined
  const title =
    kind === 'clip'
      ? t('offer.title.clip')
      : voice && missing.template
        ? t('offer.title.both')
        : voice
          ? t('offer.title.voice')
          : t('offer.title.template')

  return (
    <Sheet
      open={open}
      labelledBy={titleId}
      onClose={onClose}
      header={
        <Typography variant="title" as="h2" id={titleId}>
          {title}
        </Typography>
      }
      footer={
        <div className="flex flex-col gap-3">
          {voice && (
            <Button
              variant="cta"
              onClick={() =>
                voice.action === 'continue' ? onContinueVoice(voice.voiceId) : onCreateVoice()
              }
            >
              {voice.action === 'continue' ? t('offer.continueVoice') : t('offer.createVoice')}
            </Button>
          )}
          {missing.template && (
            <Button variant={voice ? 'secondary' : 'cta'} onClick={onCreateTemplate}>
              {kind === 'clip' ? t('offer.createClipTemplate') : t('offer.createTemplate')}
            </Button>
          )}
          <div className="flex justify-between gap-3">
            <Button variant="ghost" onClick={onDismiss}>
              {t('offer.dismiss')}
            </Button>
            <Button variant="ghost" onClick={onClose}>
              {t(`offer.close.${kind}`)}
            </Button>
          </div>
        </div>
      }
    >
      <Typography variant="body" className="text-content-secondary">
        {t(`offer.description.${kind}`)}
      </Typography>
    </Sheet>
  )
}
