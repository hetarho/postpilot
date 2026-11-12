import { useTranslation } from 'react-i18next'
import {
  AdvertiserGuideFields,
  AdvertiserGuideSuggestions,
  type AdvertiserGuide,
} from '@/entities/advertiser-guide'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Typography } from '@/shared/ui'

/** A post-specific expanded presentation. Clips keep the shared form's compact layout. */
export function CampaignMaterial({
  guide,
  refusal,
  readOnly,
  onChange,
}: {
  guide: AdvertiserGuide
  refusal: AppFailure | undefined
  readOnly: boolean
  onChange: (guide: AdvertiserGuide) => void
}) {
  const { t } = useTranslation('posts')
  return (
    <div className="mt-6">
      <Typography variant="body" as="p">
        {t('guide.requirementsHelp')}
      </Typography>
      <AdvertiserGuideFields
        value={guide}
        disabled={readOnly}
        expanded
        onChange={onChange}
        afterText={
          <AdvertiserGuideSuggestions guide={guide} readOnly={readOnly} onChange={onChange} />
        }
      />
      {refusal && (
        <div className="mt-2">
          <AppFailureMessage failure={refusal} />
        </div>
      )}
    </div>
  )
}
