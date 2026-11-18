import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { CreationOfferKind, CreationOfferMissing } from '@/features/offer-creation-setup'
import { videoTemplateIllustration } from '@/shared/assets'
import { Button, Sheet, Typography } from '@/shared/ui'
import { KitStepRow } from './KitStepRow'

interface CreationOfferSheetProps {
  kind: CreationOfferKind
  open: boolean
  missing: CreationOfferMissing | undefined
  /** A post's only: a clip is never offered a voice (CLIP-223). */
  onCreateVoice?: () => void
  onContinueVoice?: (voiceId: string) => void
  onCreateTemplate: () => void
  /** `이대로 쓰기` / `이대로 만들기`, Escape and the scrim: closed for this creation only. */
  onClose: () => void
  /** `다시 묻지 않기`. */
  onDismiss: () => void
}

/** The offer a new post or clip opens on when the account lacks what it is written with
 *  (POST-121, CLIP-223): a title naming what is missing, then one row per missing kind drawn as
 *  홈's next step (THEME-79) — the piece's ring, the action words, an arrow. It forces nothing —
 *  closing it is as plain as any action — and it holds no sibling feature: the page opens the
 *  voice sheet or routes to the wizard. */
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
        <div className="flex justify-between gap-3">
          <Button variant="ghost" onClick={onDismiss}>
            {t('offer.dismiss')}
          </Button>
          <Button variant="ghost" onClick={onClose}>
            {t(`offer.close.${kind}`)}
          </Button>
        </div>
      }
    >
      <div className="flex flex-col gap-3">
        {voice && (
          <KitStepRow
            kitKey="voice"
            // A voice being made draws as far as its meter has come; none is a dashed outline.
            state={voice.action === 'continue' ? 'partial' : 'empty'}
            fill={voice.action === 'continue' ? voice.fill : 0}
            onClick={() =>
              voice.action === 'continue' ? onContinueVoice?.(voice.voiceId) : onCreateVoice?.()
            }
          >
            {voice.action === 'continue' ? t('offer.continueVoice') : t('offer.createVoice')}
          </KitStepRow>
        )}
        {missing.template && (
          <KitStepRow
            kitKey="template"
            state="empty"
            fill={0}
            illustration={kind === 'clip' ? videoTemplateIllustration : undefined}
            onClick={onCreateTemplate}
          >
            {kind === 'clip' ? t('offer.createClipTemplate') : t('offer.createTemplate')}
          </KitStepRow>
        )}
      </div>
    </Sheet>
  )
}
