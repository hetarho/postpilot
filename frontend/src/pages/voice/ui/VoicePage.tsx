import { useMemo, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { useVoiceDetails, useVoiceProfile, voiceProfileQueryKey } from '@/entities/voice-profile'
import { StyleguideEditor, RulesEditor, StructuredProfileEditor } from '@/features/edit-voice-profile'
import { LearnVoiceForm } from '@/features/learn-voice'
import { VoiceRulesManager } from '@/features/manage-voice-rules'
import { SampleList } from '@/features/manage-voice-samples'
import { StageModelSelect } from '@/features/select-model'
import { ValidateVoiceProfile } from '@/features/validate-voice-profile'

/** Which control started the analysis that is running. Both the 학습 form and a sample deletion
 *  start one, and the two sit ~800px apart on a phone — the progress line has to render beside
 *  the control that was pressed, or it has not been shown at all (§4.3). */
type AnalysisOrigin = 'learn' | 'samples'

export function VoicePage() {
  const transport = useTransport()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { profile, isPending, isError, refetch } = useVoiceProfile(ownerId)
  const details = useVoiceDetails(ownerId)
  const [started, setStarted] = useState<{ jobId: string; origin: AnalysisOrigin } | null>(null)
  const jobId = started?.jobId || profile?.activeJobId || ''
  // An analysis resumed from the profile was not started by a press in this session; it belongs
  // to the learn section, which is where this page has always reported it.
  const origin: AnalysisOrigin = started?.origin ?? 'learn'
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

  const analysisStatus = jobId ? (
    <section className="mt-6" aria-label="문체 분석 상태">
      {jobState.isError ? (
        <FailureNotice error="문체 분석 상태를 확인하지 못했어요." onRetry={jobState.refetch} />
      ) : jobState.job?.status === 'failed' ? (
        <FailureNotice error={jobState.job.error} />
      ) : jobState.job && !isTerminal(jobState.job) ? (
        <ProgressLine job={jobState.job} />
      ) : null}
    </section>
  ) : null

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <header>
        <p className="text-content-tertiary text-xs font-medium tracking-wide uppercase">Voice</p>
        <h1 className="mt-1 text-2xl font-semibold tracking-tight">말투</h1>
        <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
          완성한 글을 직접 확정할 때마다 종결어미와 리듬을 한 편씩 배웁니다. 아무 글도 없어도 첫 글을 바로 만들 수 있어요.
        </p>
      </header>

      <StructuredProfileEditor ownerId={ownerId} profile={profile} versions={details.versions} />
      <VoiceRulesManager ownerId={ownerId} profile={profile} confirmations={details.confirmations} />
      <ValidateVoiceProfile profile={profile} />
      {details.validations.length > 0 && <section className="mt-6"><h3 className="font-medium">검증 기록</h3><ul className="mt-2 space-y-2">{details.validations.map((validation) => <li key={validation.id}><Link to="/voice/validations/$id" params={{ id: validation.id }} className="text-link-fg inline-flex min-h-11 items-center text-sm underline">v{validation.profileVersion.toString()} · {validation.status}{validation.totalCount > 0 ? ` · ${Math.round(validation.yCount / validation.totalCount * 100)}%` : ''}</Link></li>)}</ul></section>}

      <details className="mt-12">
        <summary className="min-h-11 cursor-pointer text-lg font-semibold">기존 글 가져오기 (선택)</summary>
        <p className="text-content-secondary mt-2 text-sm">이미 쓴 글을 가져오고 싶은 경우에만 사용하세요. 첫 글 생성에는 필요하지 않습니다.</p>
        <StageModelSelect stage="analyze" className="mt-4" />
        <LearnVoiceForm ownerId={ownerId} profile={profile} onStarted={(startedJobId) => setStarted({ jobId: startedJobId, origin: 'learn' })} />
        {origin === 'learn' && analysisStatus}
        <div className="mt-8"><SampleList ownerId={ownerId} samples={profile.samples} onAnalysisStarted={(startedJobId) => setStarted({ jobId: startedJobId, origin: 'samples' })} />{origin === 'samples' && analysisStatus}</div>
      </details>
      {(profile.styleguide || profile.rules) && <details className="mt-12 pb-12"><summary className="min-h-11 cursor-pointer text-lg font-semibold">이전 수동 안내</summary><p className="text-content-secondary mt-2 text-sm">기존 내용은 그대로 보존되며 생성과 AI 수정에 계속 적용됩니다.</p><div className="mt-6"><StyleguideEditor ownerId={ownerId} styleguide={profile.styleguide} /></div><div className="mt-8"><RulesEditor ownerId={ownerId} rules={profile.rules} /></div></details>}
    </main>
  )
}
