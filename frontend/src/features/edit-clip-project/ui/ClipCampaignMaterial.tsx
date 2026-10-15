import { useTranslation } from 'react-i18next'
import {
  AdvertiserGuideFields,
  EMPTY_GUIDE,
  type AdvertiserGuide,
} from '@/entities/advertiser-guide'
import { Switch, Typography } from '@/shared/ui'

export function ClipCampaignMaterial({
  campaign,
  guide = EMPTY_GUIDE,
  disabled,
  onCampaign,
  onGuide,
}: {
  campaign: boolean
  guide?: AdvertiserGuide
  disabled: boolean
  onCampaign: (on: boolean) => void
  onGuide: (guide: AdvertiserGuide) => void
}) {
  const { t } = useTranslation('clips')
  return (
    <section>
      <label className="flex min-h-11 items-center gap-3">
        <Switch
          checked={campaign}
          disabled={disabled}
          onChange={(event) => onCampaign(event.currentTarget.checked)}
        />
        {t('project.campaign')}
      </label>
      <Typography variant="meta" as="p" className="mt-1">
        {t('project.campaignHelp')}
      </Typography>
      {campaign && (
        <AdvertiserGuideFields
          value={guide}
          items={['keywords', 'hashtags']}
          disabled={disabled}
          onChange={onGuide}
        />
      )}
    </section>
  )
}
