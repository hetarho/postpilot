import { useEffect, useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import type { PostDraft } from '@/entities/post'
import { getPostQueryKey } from '@/entities/post'
import { isTerminal, ProgressLine, FailureNotice, useJob } from '@/entities/generation-job'
import { useStageSelection } from '@/entities/model-catalog'
import { voiceProfileQueryKey } from '@/entities/voice-profile'
import { Button, Dialog, FieldMessage, Notice } from '@/shared/ui'
import { useVoiceLearningActions } from '../api/useVoiceLearningActions'
import { clearLearningHandoff, readLearningHandoff, writeLearningHandoff } from '../model'

export function FinalizeAndLearn({ ownerId, post, beforeFinalize }: { ownerId: string; post: PostDraft; beforeFinalize: () => Promise<void> }) {
  const transport = useTransport()
  const analyze = useStageSelection('analyze')
  const actions = useVoiceLearningActions()
  const initial = useMemo(() => readLearningHandoff(ownerId, post.slug), [ownerId, post.slug])
  const [handoff, setHandoff] = useState(initial)
  const [confirming, setConfirming] = useState(false)
  const [preparing, setPreparing] = useState(false)
  const [satisfied, setSatisfied] = useState(false)
  const invalidate = useMemo(
    () => [getPostQueryKey(transport, post.slug), voiceProfileQueryKey(transport, ownerId)],
    [ownerId, post.slug, transport],
  )
  const jobState = useJob(handoff?.jobId ?? '', invalidate)
  const job = jobState.job
  useEffect(() => {
    if (!job || !isTerminal(job)) return
    clearLearningHandoff(ownerId, post.slug)
  }, [job, ownerId, post.slug])

  const start = async () => {
    if (!analyze.selected) return
    setPreparing(true)
    try {
      await beforeFinalize()
      const response = await actions.finalize(post.slug, analyze.selected)
      if (!response.event || !response.jobId) throw new Error('학습 작업 정보가 비어 있어요.')
      const value = { eventId: response.event.id, jobId: response.jobId }
      writeLearningHandoff(ownerId, post.slug, value)
      setHandoff(value)
      setConfirming(false)
    } finally {
      setPreparing(false)
    }
  }
  const retry = async () => {
    if (!analyze.selected || !handoff) return
    const response = await actions.retry(handoff.eventId, analyze.selected)
    if (!response.jobId) return
    const next = { ...handoff, jobId: response.jobId }
    writeLearningHandoff(ownerId, post.slug, next)
    setHandoff(next)
  }
  const noTextEdit = post.contentRevision === post.machineBaselineRevision

  return (
    <section aria-labelledby="finalize-heading" className="mt-8">
      <h2 id="finalize-heading" className="text-lg font-semibold tracking-tight">마무리</h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        최종 글을 직접 확정할 때만 이 글에서 말투를 배웁니다. 복사하거나 페이지를 열어 두는 것만으로는 학습하지 않아요.
      </p>
      {!analyze.isPending && !analyze.selected && <Notice tone="warning" role="status" className="mt-3">말투 분석 모델을 먼저 선택해 주세요.</Notice>}
      {jobState.isError ? (
        <div className="mt-3"><FailureNotice error="학습 상태를 확인하지 못했어요." onRetry={jobState.refetch} /></div>
      ) : job?.status === 'failed' ? (
        <div className="mt-3"><FailureNotice error={job.error} onRetry={() => void retry()} /></div>
      ) : job && !isTerminal(job) ? (
        <div className="mt-3"><ProgressLine job={job} /></div>
      ) : job?.status === 'done' ? (
        <Notice tone="success" role="status" className="mt-3">이 글에서 말투를 배웠어요.</Notice>
      ) : null}
      {actions.errorMessage && <FieldMessage className="mt-2">{actions.errorMessage}</FieldMessage>}
      <div className="mt-4 flex flex-wrap gap-2">
        <Button variant="cta" disabled={!post.canFinalize || !analyze.selected || Boolean(job && !isTerminal(job))} pending={preparing || actions.pending} onClick={() => setConfirming(true)}>
          확정하고 말투 학습
        </Button>
        {noTextEdit && job?.status === 'done' && !satisfied && (
          <Button variant="secondary" pending={actions.feedbackPending} onClick={() => void actions.satisfy(post.slug).then(() => setSatisfied(true))}>
            수정 없이도 마음에 들어요
          </Button>
        )}
      </div>
      <Dialog open={confirming} title="이 글을 최종본으로 확정할까요?" confirmLabel="확정하고 학습" pending={preparing || actions.pending} onClose={() => setConfirming(false)} onConfirm={() => void start()}>
        현재 편집 내용을 먼저 저장한 뒤, 선택한 분석 모델 {analyze.selected ? `${analyze.selected.providerId}/${analyze.selected.modelId}` : ''}로 학습 작업을 시작합니다. 같은 최종본을 다시 눌러도 중복 학습하지 않습니다.
      </Dialog>
    </section>
  )
}
