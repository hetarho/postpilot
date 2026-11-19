import { useTranslation } from 'react-i18next'
import {
  AdvertiserGuideFields,
  AdvertiserGuideSuggestions,
  POST_GUIDE_ITEMS,
  type AdvertiserGuide,
} from '@/entities/advertiser-guide'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Typography } from '@/shared/ui'

/** A post-specific expanded presentation. Clips keep the shared form's compact layout. A post's
 *  hashtags stay in the free text, so it offers no hashtag item or chip (POST-118). */
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
        items={POST_GUIDE_ITEMS}
        disabled={readOnly}
        expanded
        onChange={onChange}
        afterText={
          <AdvertiserGuideSuggestions
            guide={guide}
            items={POST_GUIDE_ITEMS}
            readOnly={readOnly}
            onChange={onChange}
          />
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
