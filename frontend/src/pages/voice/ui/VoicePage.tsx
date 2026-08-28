import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { useVoiceProfile, voiceProfileQueryKey } from '@/entities/voice-profile'
import { StyleguideEditor, RulesEditor } from '@/features/edit-voice-profile'
import { LearnVoiceForm } from '@/features/learn-voice'
import { SampleList } from '@/features/manage-voice-samples'
import { StageModelSelect } from '@/features/select-model'

export function VoicePage() {
  const transport = useTransport()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { profile, isPending, isError, refetch } = useVoiceProfile(ownerId)
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || profile?.activeJobId || ''
  const invalidateOnDone = useMemo(
    () => [voiceProfileQueryKey(transport, ownerId)],
    [ownerId, transport],
  )
  const jobState = useJob(jobId, invalidateOnDone)

  if (isError) {
    return (
      <main className="mx-auto w-full max-w-2xl px-4 py-10 sm:px-6">
        <FailureNotice error="문체 프로필을 불러오지 못했어요." onRetry={refetch} />
      </main>
    )
  }
  if (isPending || !profile) {
    return (
      <main className="text-content-tertiary mx-auto w-full max-w-2xl px-4 py-10 text-sm sm:px-6">
        불러오는 중…
      </main>
    )
  }

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <header>
        <p className="text-content-tertiary text-xs font-medium tracking-wide uppercase">Voice</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight">말투</h1>
        <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
          내가 쓴 글을 학습시켜 생성된 글의 종결어미와 리듬을 맞춥니다.
        </p>
      </header>

      <section className="mt-10">
        <h2 className="text-lg font-semibold tracking-tight">샘플 학습</h2>
        <StageModelSelect stage="analyze" className="mt-4" />
        <LearnVoiceForm ownerId={ownerId} profile={profile} onStarted={setStartedJobId} />
      </section>

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

      <div className="mt-12">
        <SampleList
          ownerId={ownerId}
          samples={profile.samples}
          onAnalysisStarted={setStartedJobId}
        />
      </div>
      <div className="mt-12">
        <StyleguideEditor ownerId={ownerId} styleguide={profile.styleguide} />
      </div>
      <div className="mt-12 pb-12">
        <RulesEditor ownerId={ownerId} rules={profile.rules} />
      </div>
    </main>
  )
}
