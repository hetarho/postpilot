import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { AdvertiserGuideFields, type AdvertiserGuide } from '@/entities/advertiser-guide'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Switch, Typography } from '@/shared/ui'

/** ①'s 체험단 글 switch and, while it is on, the advertiser guide (POST-117, POST-118). Off hides
 *  the guide but the draft keeps it, so turning the switch back loses nothing. */
export function CampaignMaterial({
  on,
  guide,
  refusal,
  readOnly,
  onChange,
}: {
  on: boolean
  guide: AdvertiserGuide
  /** The server's refusal of the last guide saved (POST-118), said under the guide. */
  refusal: AppFailure | undefined
  readOnly: boolean
  onChange: (next: { on: boolean; guide: AdvertiserGuide }) => void
}) {
  const { t } = useTranslation('posts')
  const id = useId()
  return (
    <section aria-labelledby={`${id}-label`} className="mt-6">
      {/* The whole row is the switch's label, so it keeps the 44px touch floor (THEME-23). */}
      <label className={`flex min-h-11 items-center gap-3 ${readOnly ? '' : 'cursor-pointer'}`}>
        <Typography variant="fieldTitle" as="span" id={`${id}-label`} className="min-w-0 flex-1">
          {t('advertiserGuide.campaign')}
        </Typography>
        <Switch
          aria-describedby={`${id}-help`}
          checked={on}
          disabled={readOnly}
          onChange={(event) => onChange({ on: event.target.checked, guide })}
        />
      </label>
      <Typography variant="meta" as="p" id={`${id}-help`}>
        {t('advertiserGuide.campaignHelp')}
      </Typography>
      {on && (
        <AdvertiserGuideFields
          value={guide}
          disabled={readOnly}
          onChange={(next) => onChange({ on, guide: next })}
        />
      )}
      {refusal && (
        <div className="mt-2">
          <AppFailureMessage failure={refusal} />
        </div>
      )}
    </section>
  )
}
