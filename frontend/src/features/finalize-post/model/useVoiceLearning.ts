import i18next from 'i18next'
import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { isTerminal, useJob, type GenerationJob } from '@/entities/generation-job'
import { useStageSelection } from '@/entities/model-catalog'
import { getPostQueryKey, type PostDraft } from '@/entities/post'
import {
  deletedVoiceAIReason,
  voiceContentLanguageMismatch,
  voiceContentLanguageMismatchReason,
  voiceProfileQueryKey,
} from '@/entities/voice'
import { useVoiceLearningActions } from '../api/useVoiceLearningActions'
import {
  isLearningHandoffForRevision,
  readLearningHandoff,
  writeLearningHandoff,
} from './learning-handoff'

export interface VoiceLearning {
  /** The learning job this revision started, once there is one. */
  job: GenerationJob | undefined
  /** The job's own state could not be read — not the job failing. */
  isError: boolean
  refetch: () => void
  /** A learning run for this revision is queued or running. */
  active: boolean
  /** This revision has already been learned from, so learning it again would say nothing new. */
  learned: boolean
  /** The last run for this revision failed and can be retried without re-finalizing. */
  retryable: boolean
  /** Why this post can never be learned from as it stands, or '' when it can. */
  blocked: string
  /** An analyze model is chosen and the voice gates pass — true before the post is finalized,
   *  because 확정하고 말투 학습 finalizes first and learns second. */
  canLearn: boolean
  /** The server's own precondition: the exact revision on screen is the finalized one. */
  revisionFinalized: boolean
  /** `canLearn` and `revisionFinalized` together — the learn button's whole gate. */
  readyToLearn: boolean
  /** No analyze model is selected yet — the one gate the user fixes on another screen. */
  needsAnalyzeModel: boolean
  pending: boolean
  errorMessage: string
  /** The user finalized the machine draft without touching a word of it. */
  noTextEdit: boolean
  feedbackPending: boolean
  satisfied: boolean
  learn: (revision?: bigint) => Promise<void>
  retry: () => Promise<void>
  satisfy: () => Promise<void>
}

/** The single owner of "what has this post's current revision taught the voice, if anything".
 *
 *  A hook rather than a component because its two surfaces sit on different step panels —
 *  확정하고 말투 학습 at the end of 글 다듬기, the 말투 학습 button and every learning outcome on
 *  글 완성. Finalizing switches the step, so a component owning this state would be unmounted
 *  mid-flight by the very action that starts the run. Held above both panels, the started job,
 *  its handoff and its error all survive the switch. */
export function useVoiceLearning(ownerId: string, post: PostDraft): VoiceLearning {
  const transport = useTransport()
  const analyze = useStageSelection('analyze')
  const actions = useVoiceLearningActions()
  const [handoff, setHandoff] = useState(() => readLearningHandoff(ownerId, post.slug))
  const [satisfied, setSatisfied] = useState(false)
  // A handoff from an older revision is not this revision's answer: the content has moved on, so
  // the post is learnable again and the completed run must not say otherwise.
  const current = isLearningHandoffForRevision(handoff, post.contentRevision)
  // The learning event publishes to the post's voice and no other, so that is the one profile
  // the completed job makes stale.
  const invalidate = useMemo(
    () => [
      getPostQueryKey(transport, post.slug),
      voiceProfileQueryKey(transport, ownerId, post.voice.id),
    ],
    [ownerId, post.slug, post.voice.id, transport],
  )
  const jobState = useJob(current ? (handoff?.jobId ?? '') : '', invalidate)
  const job = jobState.job
  const active = Boolean(job && !isTerminal(job))
  const learned = current && job?.status === 'done'

  // Mirrors the server's two voice gates on learning (tech/multi-voice-partitioning.md): a
  // deleted voice cannot receive evidence, and a baseline written under another voice — a post
  // reassigned since generation — must not be read as a correction of the new one. Finalizing
  // itself stays available: it is a content boundary, not a profile mutation.
  const hasLearnableBaseline =
    post.machineBaselineRevision > 0n && post.machineBaselineVoiceId === post.voice.id
  const blocked = post.voice.deleted
    ? deletedVoiceAIReason()
    : voiceContentLanguageMismatch(post.contentLanguage, post.voice.sourceLanguage)
      ? voiceContentLanguageMismatchReason()
      : post.machineBaselineVoiceId !== '' && post.machineBaselineVoiceId !== post.voice.id
        ? i18next.t('learning.blocked.otherVoice', { ns: 'posts' })
        : !hasLearnableBaseline
          ? i18next.t('learning.blocked.noBaseline', { ns: 'posts' })
          : ''
  const canLearn = !blocked && Boolean(analyze.selected) && !active && !learned
  // The server refuses learning unless the exact revision on screen is the finalized one
  // (policy/voice.md), so the button says so rather than offering a call that would fail.
  const finalizedNow =
    post.status === 'finalized' && post.finalizedRevision === post.contentRevision

  const learn = async (revision = post.contentRevision) => {
    if (!analyze.selected) return
    const response = await actions.learn(post.slug, analyze.selected)
    if (!response.event || !response.jobId)
      throw new Error(i18next.t('learning.missingJob', { ns: 'posts' }))
    // The revision the caller actually finalized, not the one this render happened to see: a
    // flush immediately before the finalize can have moved it.
    const next = {
      eventId: response.event.id,
      jobId: response.jobId,
      contentRevision: revision.toString(),
    }
    writeLearningHandoff(ownerId, post.slug, next)
    setHandoff(next)
  }

  const retry = async () => {
    // A failed job can outlive the voice/content pairing that was eligible when it started.
    // Re-evaluate the current language, deletion and baseline gates before any retry RPC.
    if (blocked || !analyze.selected || !handoff) return
    const response = await actions.retry(handoff.eventId, analyze.selected)
    if (!response.jobId) return
    const next = { ...handoff, jobId: response.jobId }
    writeLearningHandoff(ownerId, post.slug, next)
    setHandoff(next)
  }

  return {
    job,
    isError: jobState.isError,
    refetch: jobState.refetch,
    active,
    learned,
    retryable: !blocked && current && job?.status === 'failed',
    blocked,
    canLearn,
    revisionFinalized: finalizedNow,
    readyToLearn: canLearn && finalizedNow,
    needsAnalyzeModel: !blocked && !analyze.isPending && !analyze.selected,
    pending: actions.pending,
    errorMessage: actions.errorMessage,
    noTextEdit: post.contentRevision === post.machineBaselineRevision,
    feedbackPending: actions.feedbackPending,
    satisfied,
    learn,
    retry,
    satisfy: () => actions.satisfy(post.slug).then(() => setSatisfied(true)),
  }
}
