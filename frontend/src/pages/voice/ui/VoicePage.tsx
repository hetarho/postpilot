import { useTranslation } from 'react-i18next'
import { FailureNotice } from '@/entities/generation-job'
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
          {/* The run's own status ends with the job; the server keeps a failed seed on the
              profile until a version is published, so it still says why after a reload
              (VOICE-19). 기존 글 가져오기 is the way on. */}
          {!profile.activeJobId && profile.seedFailure && (
            <section className="mt-6" aria-label={t('screens.analysisStatus', { ns: 'voices' })}>
              <FailureNotice failure={profile.seedFailure} />
            </section>
          )}
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
