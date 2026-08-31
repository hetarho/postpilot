import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { voiceProfileQueryKey } from '@/entities/voice'
import { RulesEditor, StyleguideEditor } from '@/features/edit-voice-profile'
import { LearnVoiceForm } from '@/features/learn-voice'
import { SampleList } from '@/features/manage-voice-samples'
import { StageModelSelect } from '@/features/select-model'
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
  const transport = useTransport()
  // This screen is the only one that starts an analysis, so the progress line has exactly one
  // place to be.
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || profile.activeJobId
  const invalidateOnDone = useMemo(
    () => [voiceProfileQueryKey(transport, ownerId, voiceId)],
    [ownerId, transport, voiceId],
  )
  const jobState = useJob(jobId, invalidateOnDone)
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
      {jobId && (
        <section className="mt-6" aria-label={t('screens.analysisStatus')}>
          {jobState.isError ? (
            <FailureNotice message={t('screens.analysisStatusFailed')} onRetry={jobState.refetch} />
          ) : jobState.job?.status === 'failed' ? (
            <FailureNotice failure={jobState.job.failure} />
          ) : jobState.job && !isTerminal(jobState.job) ? (
            <ProgressLine job={jobState.job} />
          ) : null}
        </section>
      )}
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
          <h3 className="text-lg font-semibold tracking-tight">{t('screens.previousGuidance')}</h3>
          <p className="text-content-secondary mt-2 text-sm">{t('screens.previousGuidanceHelp')}</p>
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
