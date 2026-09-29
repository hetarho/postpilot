import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { VoiceReadinessMeter } from '@/entities/voice'
import { StructuredProfileEditor } from '@/features/edit-voice-profile'
import { typographyStyles } from '@/shared/ui'
import { VoiceRunStatus } from './VoiceRunStatus'
import { VoiceScreen } from './VoiceScreen'

export function VoicePage() {
  const { t } = useTranslation(['voices', 'nav'])
  return (
    <VoiceScreen title={t('voice.profile', { ns: 'nav' })}>
      {({ profile, voice, ownerId, voiceId }) => (
        <>
          {/* Until the voice is made there is no analysis to read: the meter and the way to
              학습 글 stand in its place (VOICE-63). */}
          {!profile.made && (
            <div className="mb-6">
              <VoiceReadinessMeter readiness={profile.readiness} />
              <Link
                to="/voices/$voiceId/materials"
                params={{ voiceId }}
                className={typographyStyles({
                  variant: 'label',
                  className:
                    'text-link-fg hover:text-link-fg-hover mt-2 inline-flex min-h-11 items-center underline',
                })}
              >
                {t('screens.toMaterials', { ns: 'voices' })}
              </Link>
            </div>
          )}
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
