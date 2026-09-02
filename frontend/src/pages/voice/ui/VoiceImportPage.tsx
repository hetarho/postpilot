import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RulesEditor, StyleguideEditor } from '@/features/edit-voice-profile'
import { LearnVoiceForm } from '@/features/learn-voice'
import { SampleList } from '@/features/manage-voice-samples'
import { StageModelSelect } from '@/features/select-model'
import { Typography } from '@/shared/ui'
import { VoiceRunStatus } from './VoiceRunStatus'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

export function VoiceImportPage() {
  const { t } = useTranslation('voices')
  return (
    <VoiceScreen title={t('screens.importTitle')} description={t('screens.importDescription')}>
      {(context) => <ImportPanel {...context} />}
    </VoiceScreen>
  )
}

function ImportPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { t } = useTranslation('voices')
  // The id the just-started analysis returned outruns the profile refetch that will carry it,
  // so this screen holds it until the query catches up.
  const [startedJobId, setStartedJobId] = useState('')
  const blocked = voice.deleted ? t('screens.importBlocked') : ''

  return (
    <>
      <StageModelSelect stage="analyze" />
      <LearnVoiceForm
        ownerId={ownerId}
        voiceId={voiceId}
        profile={profile}
        onStarted={setStartedJobId}
        blocked={blocked}
      />
      <VoiceRunStatus
        ownerId={ownerId}
        voiceId={voiceId}
        jobId={startedJobId || profile.activeJobId}
      />
      <div className="mt-8">
        <SampleList
          ownerId={ownerId}
          voiceId={voiceId}
          samples={profile.samples}
          onAnalysisStarted={setStartedJobId}
          blocked={voice.deleted}
        />
      </div>
      {(profile.styleguide || profile.rules) && (
        <section className="mt-12">
          <Typography variant="title" as="h3">
            {t('screens.previousGuidance')}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-2">
            {t('screens.previousGuidanceHelp')}
          </Typography>
          <div className="mt-6">
            <StyleguideEditor
              ownerId={ownerId}
              voiceId={voiceId}
              styleguide={profile.styleguide}
              readOnly={voice.deleted}
            />
          </div>
          <div className="mt-8">
            <RulesEditor
              ownerId={ownerId}
              voiceId={voiceId}
              rules={profile.rules}
              readOnly={voice.deleted}
            />
          </div>
        </section>
      )}
    </>
  )
}
