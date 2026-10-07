import { useTranslation } from 'react-i18next'
import { Badge, Notice, Typography } from '@/shared/ui'
import { voiceMaterialFreshness, type VoiceProfile } from '../model/types'

/** A pending or unprovable source snapshot never makes the accepted writing style unusable. */
export function VoiceMaterialFreshness({
  profile,
  compact = false,
}: {
  profile: VoiceProfile
  compact?: boolean
}) {
  const { t } = useTranslation('voices')
  const freshness = voiceMaterialFreshness(profile)
  if (freshness === 'unmade' || freshness === 'current') return null
  if (compact)
    return (
      <Badge tone="warning">
        {t(freshness === 'pending' ? 'freshness.pendingRow' : 'freshness.unknownRow')}
      </Badge>
    )
  return (
    <Notice tone="info" role="status">
      <Typography variant="label" as="p">
        {t(freshness === 'pending' ? 'freshness.pending' : 'freshness.unknown')}
      </Typography>
      <Typography variant="body" as="p" className="mt-1">
        {t('freshness.previous')}
      </Typography>
      {profile.activeJobId && (
        <Typography variant="body" as="p" className="mt-1">
          {t('freshness.concurrent')}
        </Typography>
      )}
    </Notice>
  )
}
