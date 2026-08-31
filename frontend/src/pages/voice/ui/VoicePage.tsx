import { useTranslation } from 'react-i18next'
import { StructuredProfileEditor } from '@/features/edit-voice-profile'
import { VoiceScreen } from './VoiceScreen'

export function VoicePage() {
  const { t } = useTranslation(['voices', 'nav'])
  return (
    <VoiceScreen
      title={t('voice.profile', { ns: 'nav' })}
      description={t('screens.profileDescription', { ns: 'voices' })}
    >
      {({ profile, voice, ownerId, voiceId }) => (
        <StructuredProfileEditor
          ownerId={ownerId}
          voiceId={voiceId}
          profile={profile}
          sourceLanguage={voice.sourceLanguage}
          readOnly={voice.deleted}
        />
      )}
    </VoiceScreen>
  )
}
