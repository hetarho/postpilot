import { useEffect, useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import type { PostDraft } from '@/entities/post'
import { getPostQueryKey } from '@/entities/post'
import { FailureNotice, isTerminal, ProgressLine, useJob } from '@/entities/generation-job'
import { useStageSelection } from '@/entities/model-catalog'
import { DELETED_VOICE_AI_REASON, voiceProfileQueryKey } from '@/entities/voice'
import { Button, Dialog, FieldMessage, Notice } from '@/shared/ui'
import { useFinalizePost } from '../api/useFinalizePost'
import { useVoiceLearningActions } from '../api/useVoiceLearningActions'
import {
  clearLearningHandoff,
  isLearningHandoffForRevision,
  readLearningHandoff,
  writeLearningHandoff,
} from '../model/learning-handoff'

type FinalizeMode = 'finalize' | 'learn'

export function FinalizePost({
  ownerId,
  post,
  beforeFinalize,
}: {
  ownerId: string
  post: PostDraft
  beforeFinalize: () => Promise<bigint>
}) {
  const transport = useTransport()
  const analyze = useStageSelection('analyze')
  const finalize = useFinalizePost()
  const learning = useVoiceLearningActions()
  const initial = useMemo(() => readLearningHandoff(ownerId, post.slug), [ownerId, post.slug])
  const [handoff, setHandoff] = useState(initial)
  const [confirming, setConfirming] = useState<FinalizeMode | ''>('')
  const [preparing, setPreparing] = useState<FinalizeMode | ''>('')
  const [satisfied, setSatisfied] = useState(false)
  const handoffIsCurrent = isLearningHandoffForRevision(handoff, post.contentRevision)
  // The learning event publishes to the post's voice and no other, so that is the one profile
  // the completed job makes stale.
  const invalidate = useMemo(
    () => [
      getPostQueryKey(transport, post.slug),
      voiceProfileQueryKey(transport, ownerId, post.voice.id),
    ],
    [ownerId, post.slug, post.voice.id, transport],
  )
  const jobState = useJob(handoffIsCurrent ? (handoff?.jobId ?? '') : '', invalidate)
  const job = jobState.job
  useEffect(() => {
    if (job?.status !== 'done') return
    clearLearningHandoff(ownerId, post.slug)
  }, [job, ownerId, post.slug])

  const learn = async () => {
    if (!analyze.selected) return
    const response = await learning.learn(post.slug, analyze.selected)
    if (!response.event || !response.jobId) throw new Error('학습 작업 정보가 비어 있어요.')
    const next = {
      eventId: response.event.id,
      jobId: response.jobId,
      contentRevision: post.contentRevision.toString(),
    }
    writeLearningHandoff(ownerId, post.slug, next)
    setHandoff(next)
  }

  const run = async (mode: FinalizeMode) => {
    if (mode === 'learn' && !analyze.selected) return
    setPreparing(mode)
    try {
      const revision = await beforeFinalize()
      await finalize.finalize(post.slug, revision)
      setConfirming('')
      if (mode === 'learn') await learn()
    } catch {
      // Each mutation renders its own error. A learning failure intentionally does
      // not undo the finalized post written immediately before it.
    } finally {
      setPreparing('')
    }
  }

  const retry = async () => {
    if (!analyze.selected || !handoff) return
    const response = await learning.retry(handoff.eventId, analyze.selected)
    if (!response.jobId) return
    const next = { ...handoff, jobId: response.jobId }
    writeLearningHandoff(ownerId, post.slug, next)
    setHandoff(next)
  }
  const learningActive = Boolean(job && !isTerminal(job))
  const finalized = post.status === 'finalized'
  const learnedCurrent = handoffIsCurrent && job?.status === 'done'
  const noTextEdit = post.contentRevision === post.machineBaselineRevision
  const hasLearnableBaseline =
    post.machineBaselineRevision > 0n && post.machineBaselineVoiceId === post.voice.id
  // Mirrors the server's two voice gates on learning (tech/multi-voice-partitioning.md): a
  // deleted voice cannot receive evidence, and a baseline written under another voice — a post
  // reassigned since generation — must not be read as a correction of the new one. Finalizing
  // itself stays available: it is a content boundary, not a profile mutation.
  const learnBlocked = post.voice.deleted
    ? DELETED_VOICE_AI_REASON
    : post.machineBaselineVoiceId !== '' && post.machineBaselineVoiceId !== post.voice.id
      ? '이 글의 AI 결과는 다른 말투에서 만들어졌어요. 새 말투로 다시 생성하거나 수정한 뒤에 학습할 수 있어요.'
      : !hasLearnableBaseline
        ? '새 말투로 다시 생성하거나 AI로 수정한 뒤에 학습할 수 있어요.'
        : ''
  const canLearn = !learnBlocked && Boolean(analyze.selected) && !learningActive

  return (
    <section aria-labelledby="finalize-heading" className="mt-8">
      <h2 id="finalize-heading" className="text-lg font-semibold tracking-tight">
        마무리
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        확정과 말투 학습은 별개예요. 확정만 해도 글은 완료되며, 학습은 버튼을 눌렀을 때만
        시작합니다.
      </p>
      {jobState.isError ? (
        <div className="mt-3">
          <FailureNotice error="학습 상태를 확인하지 못했어요." onRetry={jobState.refetch} />
        </div>
      ) : job?.status === 'failed' ? (
        <div className="mt-3">
          <FailureNotice error={job.error} onRetry={() => void retry()} />
        </div>
      ) : learningActive && job ? (
        <div className="mt-3">
          <ProgressLine job={job} />
        </div>
      ) : job?.status === 'done' || learnedCurrent ? (
        <Notice tone="success" role="status" className="mt-3">
          이 글에서 말투를 배웠어요.
        </Notice>
      ) : finalized ? (
        <Notice tone="success" role="status" className="mt-3">
          이 revision을 확정했어요.
        </Notice>
      ) : null}
      {finalize.isError && <FieldMessage className="mt-2">글을 확정하지 못했어요.</FieldMessage>}
      {learning.errorMessage && (
        <FieldMessage className="mt-2">
          글은 확정됐지만 말투 학습은 시작하지 못했어요. {learning.errorMessage}
        </FieldMessage>
      )}
      {learnBlocked ? (
        <p role="status" className="text-content-secondary mt-2 text-sm">
          {learnBlocked}
        </p>
      ) : (
        !analyze.isPending &&
        !analyze.selected && (
          <p className="text-content-tertiary mt-2 text-sm">
            말투 학습을 하려면 분석 모델을 선택해 주세요. 확정만 하는 데에는 필요하지 않아요.
          </p>
        )
      )}
      <div className="mt-4 flex flex-wrap gap-2">
        {!finalized && (
          <>
            <Button
              variant="secondary"
              disabled={!post.canFinalize || learningActive}
              pending={preparing === 'finalize'}
              onClick={() => setConfirming('finalize')}
            >
              확정
            </Button>
            <Button
              variant="cta"
              disabled={!post.canFinalize || !canLearn}
              pending={preparing === 'learn'}
              onClick={() => setConfirming('learn')}
            >
              확정하고 말투 학습
            </Button>
          </>
        )}
        {finalized && (!handoff || !handoffIsCurrent) && !learnedCurrent && (
          <Button
            variant="cta"
            disabled={!canLearn}
            pending={learning.pending}
            onClick={() => void learn().catch(() => undefined)}
          >
            말투 학습
          </Button>
        )}
        {noTextEdit && learnedCurrent && !satisfied && (
          <Button
            variant="secondary"
            pending={learning.feedbackPending}
            onClick={() => void learning.satisfy(post.slug).then(() => setSatisfied(true))}
          >
            수정 없이도 마음에 들어요
          </Button>
        )}
      </div>
      <Dialog
        open={Boolean(confirming)}
        title="이 revision을 확정할까요?"
        confirmLabel={confirming === 'learn' ? '확정하고 학습' : '확정'}
        pending={Boolean(preparing)}
        onClose={() => setConfirming('')}
        onConfirm={() => confirming && void run(confirming)}
      >
        현재 편집 내용을 먼저 저장한 뒤 정확한 revision을 확정합니다.
        {confirming === 'learn'
          ? ' 그 다음에만 말투 학습을 시작합니다.'
          : ' 모델 호출이나 말투 학습은 하지 않습니다.'}
      </Dialog>
    </section>
  )
}
