import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { voiceProfileQueryKey } from '@/entities/voice'
import { RulesEditor, StyleguideEditor } from '@/features/edit-voice-profile'
import { LearnVoiceForm } from '@/features/learn-voice'
import { SampleList } from '@/features/manage-voice-samples'
import { StageModelSelect } from '@/features/select-model'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

export function VoiceImportPage() {
  return (
    <VoiceScreen
      title="기존 글 가져오기"
      description="이미 쓴 글을 이 말투에 가져오고 싶은 경우에만 사용하세요. 첫 글 생성에는 필요하지 않고, 다른 말투의 글은 가져올 수 없어요."
    >
      {(context) => <ImportPanel {...context} />}
    </VoiceScreen>
  )
}

function ImportPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
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
  const blocked = voice.deleted ? '삭제된 말투에는 글을 가져올 수 없어요. 먼저 복원해 주세요.' : ''

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
        <section className="mt-6" aria-label="문체 분석 상태">
          {jobState.isError ? (
            <FailureNotice error="문체 분석 상태를 확인하지 못했어요." onRetry={jobState.refetch} />
          ) : jobState.job?.status === 'failed' ? (
            <FailureNotice error={jobState.job.error} />
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
          <h3 className="text-lg font-semibold tracking-tight">이전 수동 안내</h3>
          <p className="text-content-secondary mt-2 text-sm">
            기존 내용은 그대로 보존되며 이 말투의 생성과 AI 수정에 계속 적용됩니다.
          </p>
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
