import { useTranslation } from 'react-i18next'
import { StructuredProfileEditor } from '@/features/edit-voice-profile'
import { VoiceRunStatus } from './VoiceRunStatus'
import { VoiceScreen } from './VoiceScreen'

export function VoicePage() {
  const { t } = useTranslation(['voices', 'nav'])
  return (
    <VoiceScreen
      title={t('voice.profile', { ns: 'nav' })}
      description={t('screens.profileDescription', { ns: 'voices' })}
    >
      {({ profile, voice, ownerId, voiceId }) => (
        <>
          <VoiceRunStatus ownerId={ownerId} voiceId={voiceId} jobId={profile.activeJobId} />
          <StructuredProfileEditor
            ownerId={ownerId}
            voiceId={voiceId}
            profile={profile}
            readOnly={voice.deleted}
          />
        </>
      )}
    </VoiceScreen>
  )
}
