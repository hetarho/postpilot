import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { type VoiceProfile, voiceProfileQueryKey } from '@/entities/voice-profile'
import { RulesEditor, StyleguideEditor } from '@/features/edit-voice-profile'
import { LearnVoiceForm } from '@/features/learn-voice'
import { SampleList } from '@/features/manage-voice-samples'
import { StageModelSelect } from '@/features/select-model'
import { VoiceScreen } from './VoiceScreen'

export function VoiceImportPage() {
  return (
    <VoiceScreen
      title="기존 글 가져오기"
      description="이미 쓴 글을 가져오고 싶은 경우에만 사용하세요. 첫 글 생성에는 필요하지 않습니다."
    >
      {({ profile, ownerId }) => <ImportPanel ownerId={ownerId} profile={profile} />}
    </VoiceScreen>
  )
}

function ImportPanel({ ownerId, profile }: { ownerId: string; profile: VoiceProfile }) {
  const transport = useTransport()
  // This screen is now the only one that starts an analysis, so the progress line has exactly one
  // place to be — the two-origin branch that used to place it beside whichever control was pressed
  // is gone with the split (§4.3).
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || profile.activeJobId
  const invalidateOnDone = useMemo(
    () => [voiceProfileQueryKey(transport, ownerId)],
    [ownerId, transport],
  )
  const jobState = useJob(jobId, invalidateOnDone)

  return (
    <>
      <StageModelSelect stage="analyze" />
      <LearnVoiceForm ownerId={ownerId} profile={profile} onStarted={setStartedJobId} />
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
          samples={profile.samples}
          onAnalysisStarted={setStartedJobId}
        />
      </div>
      {(profile.styleguide || profile.rules) && (
        <section className="mt-12">
          <h2 className="text-lg font-semibold tracking-tight">이전 수동 안내</h2>
          <p className="text-content-secondary mt-2 text-sm">
            기존 내용은 그대로 보존되며 생성과 AI 수정에 계속 적용됩니다.
          </p>
          <div className="mt-6">
            <StyleguideEditor ownerId={ownerId} styleguide={profile.styleguide} />
          </div>
          <div className="mt-8">
            <RulesEditor ownerId={ownerId} rules={profile.rules} />
          </div>
        </section>
      )}
    </>
  )
}
