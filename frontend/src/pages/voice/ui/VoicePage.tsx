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
          {/* This is the tab a newly created voice lands on, so the seeding run its creation
              started reports here — and so does its failure, which is the only thing that says
              the described profile is not coming. */}
          <VoiceRunStatus ownerId={ownerId} voiceId={voiceId} jobId={profile.activeJobId} />
          <StructuredProfileEditor
            ownerId={ownerId}
            voiceId={voiceId}
            profile={profile}
            sourceLanguage={voice.sourceLanguage}
            readOnly={voice.deleted}
          />
        </>
      )}
    </VoiceScreen>
  )
}
