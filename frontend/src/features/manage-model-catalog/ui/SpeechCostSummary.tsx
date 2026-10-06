import { useTranslation } from 'react-i18next'
import type { AdminSpeechProfile } from '@/entities/model-catalog'
import { Typography } from '@/shared/ui'
import { SPEECH_PUBLIC_PRICING } from '../config/speech'
import { publicSpeechPrice, speechOperationCost } from '../lib/speech-pricing'

export function SpeechCostSummary({ profile }: { profile: AdminSpeechProfile }) {
  const { t } = useTranslation('models')
  const characters = SPEECH_PUBLIC_PRICING.characters
  const accountDesign = speechOperationCost(
    profile.prices.find((p) => p.operation === 'voice_design'),
    characters,
  )
  const accountSpeech = speechOperationCost(
    profile.prices.find((p) => p.operation === 'speech'),
    characters,
  )
  const accountConfirm = speechOperationCost(
    profile.prices.find((p) => p.operation === 'voice_confirm'),
    characters,
  )
  const publicPrice = publicSpeechPrice(profile.binding.speechModel.modelId)
  const speechUsd = accountSpeech ?? publicPrice?.usd
  return (
    <div className="space-y-1">
      <Typography variant="label">{t('speechAdmin.costTitle')}</Typography>
      <Typography variant="meta">
        {t('speechAdmin.designEstimate', {
          characters,
          usd: accountDesign ?? SPEECH_PUBLIC_PRICING.designUsd,
        })}
      </Typography>
      <Typography variant="meta">
        {t('speechAdmin.confirmEstimate', {
          usd: accountConfirm ?? SPEECH_PUBLIC_PRICING.confirmationUsd,
        })}
      </Typography>
      <Typography variant="meta">
        {speechUsd !== undefined
          ? t('speechAdmin.speechEstimate', { characters, usd: speechUsd })
          : t('speechAdmin.speechEstimateUnknown')}
      </Typography>
      {accountSpeech === null && publicPrice?.promotionActive && (
        <Typography variant="meta">
          {t('speechAdmin.publicPromotion', {
            until: SPEECH_PUBLIC_PRICING.promotionUntil,
            usd: publicPrice.standardUsd,
          })}
        </Typography>
      )}
      {accountDesign === null && (
        <Typography variant="meta">{t('speechAdmin.designEstimateBasis')}</Typography>
      )}
      <Typography variant="meta">
        {t(
          accountDesign !== null && accountSpeech !== null && accountConfirm !== null
            ? 'speechAdmin.accountEstimateBasis'
            : 'speechAdmin.publicEstimateBasis',
          { at: SPEECH_PUBLIC_PRICING.reviewedAt },
        )}
      </Typography>
      <Typography
        as="a"
        variant="meta"
        href={SPEECH_PUBLIC_PRICING.source}
        target="_blank"
        rel="noreferrer"
        className="text-content-secondary underline"
      >
        {t('speechAdmin.publicPriceSource')}
      </Typography>
    </div>
  )
}
