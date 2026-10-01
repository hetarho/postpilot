import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { VoiceReadinessMeter } from '@/entities/voice'
import { AnswerPromptsSheet } from '@/features/answer-voice-prompt'
import { MakeVoiceButton } from '@/features/make-voice'
import { SampleList } from '@/features/manage-voice-samples'
import { PasteMaterialSheet } from '@/features/paste-voice-material'
import { Notice } from '@/shared/ui'
import { VoiceRunStatus } from './VoiceRunStatus'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

/** The 학습 글 tab (VOICE-64): the readiness meter until the voice is made, 말투 만들기 or 다시
 *  분석, the two ways to add 학습 글, and the list. */
export function VoiceMaterialsPage() {
  const { t } = useTranslation('nav')
  return (
    <VoiceScreen title={t('voice.materials')}>
      {(context) => <MaterialsPanel {...context} />}
    </VoiceScreen>
  )
}

function MaterialsPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { t } = useTranslation('voices')
  // The id the just-started analysis returned outruns the profile refetch that will carry it,
  // so this screen holds it until the query catches up.
  const [startedJobId, setStartedJobId] = useState('')
  return (
    <>
      {!profile.made && <VoiceReadinessMeter readiness={profile.readiness} />}
      <div className="mt-4">
        <MakeVoiceButton
          ownerId={ownerId}
          voiceId={voiceId}
          profile={profile}
          onStarted={setStartedJobId}
        />
      </div>
      <VoiceRunStatus
        ownerId={ownerId}
        voiceId={voiceId}
        jobId={startedJobId || profile.activeJobId}
      />
      {voice.deleted && (
        <Notice tone="warning" role="status" className="mt-6">
          {t('screens.materialsBlocked')}
        </Notice>
      )}
      <div className="mt-8 flex flex-wrap gap-2">
        <PasteMaterialSheet ownerId={ownerId} voiceId={voiceId} disabled={voice.deleted} />
        <AnswerPromptsSheet
          ownerId={ownerId}
          voiceId={voiceId}
          samples={profile.samples}
          readiness={profile.made ? undefined : profile.readiness}
          disabled={voice.deleted}
        />
      </div>
      <div className="mt-8">
        <SampleList
          ownerId={ownerId}
          voiceId={voiceId}
          samples={profile.samples}
          blocked={voice.deleted}
        />
      </div>
    </>
  )
}
