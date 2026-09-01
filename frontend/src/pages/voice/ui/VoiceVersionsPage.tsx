import { useTranslation } from 'react-i18next'
import { useVoiceVersions } from '@/entities/voice'
import { VoiceVersionHistory } from '@/features/edit-voice-profile'
import { Typography } from '@/shared/ui'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

export function VoiceVersionsPage() {
  const { t } = useTranslation(['voices', 'nav'])
  return (
    <VoiceScreen
      title={t('voice.versions', { ns: 'nav' })}
      description={t('screens.versionsDescription', { ns: 'voices' })}
    >
      {(context) => <VersionList {...context} />}
    </VoiceScreen>
  )
}

/** Its own component so the versions query is issued by the screen that renders it — mounting the
 *  profile tab must not fetch this list. */
function VersionList({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { t } = useTranslation('common')
  const { versions, isPending } = useVoiceVersions(ownerId, voiceId)
  // "아직 저장된 버전이 없어요" while the list is still in flight would be a claim about the
  // voice, not about the request.
  if (isPending)
    return (
      <Typography variant="body" className="text-content-tertiary">
        {t('state.loading')}
      </Typography>
    )
  return (
    <VoiceVersionHistory
      ownerId={ownerId}
      voiceId={voiceId}
      profile={profile}
      versions={versions}
      readOnly={voice.deleted}
    />
  )
}
