import { forwardRef } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { ReviseForm, type ReviseFormHandle } from '@/features/edit-with-ai'
import { FinalizeActions, type VoiceLearning } from '@/features/finalize-post'

interface RefineDockProps {
  ownerId: string
  post: PostDraft
  /** True when the post's content language and its voice's source language disagree: the revision
   *  still runs, but its instruction may not be published as a voice rule. */
  ruleLanguageMismatch: boolean
  /** The voice-learning run. It lives ABOVE the step panels in `pages/editor`, because
   *  확정하고 말투 학습 starts it here and every outcome is reported on 글 완성 — so the run has to
   *  outlive the step change the finalize itself causes. */
  learning: VoiceLearning
  activeJob?: GenerationJob
  jobPending: boolean
  onRevisionStarted: (jobId: string) => void
  /** Flushes a pending block edit. Both rows take it: a finalize may never name a revision that
   *  omits an edit the user has already made. */
  beforeStart: () => Promise<void>
  beforeFinalize: () => Promise<bigint>
  /** Carries the title 확정 wrote into `posts.title`, so the editor can re-seed its 가제. */
  onFinalized: (title: string) => void
}

/** The body of 글 다듬기's dock: **row 1** is the revision instruction with its send button, and
 *  **row 2** is 확정 beside 확정하고 말투 학습.
 *
 *  It is a WIDGET because it composes two sibling `features/*` slices — `edit-with-ai` and
 *  `finalize-post` — and a feature may not import a sibling (ARCHITECTURE §3).
 *
 *  Each row renders its own blockers, validation and failures directly above its own controls
 *  (§8.3): the keyboard covers roughly the bottom 40% of the screen, so it may hide a control but
 *  never the reason that control is disabled. */
export const RefineDock = forwardRef<ReviseFormHandle, RefineDockProps>(function RefineDock(
  {
    ownerId,
    post,
    ruleLanguageMismatch,
    learning,
    activeJob,
    jobPending,
    onRevisionStarted,
    beforeStart,
    beforeFinalize,
    onFinalized,
  },
  reviseRef,
) {
  return (
    <div className="grid gap-3">
      <ReviseForm
        ref={reviseRef}
        ownerId={ownerId}
        postSlug={post.slug}
        voice={post.voice}
        ruleLanguageMismatch={ruleLanguageMismatch}
        purpose={post.purpose}
        activeJob={activeJob}
        jobPending={jobPending}
        onStarted={onRevisionStarted}
        beforeStart={beforeStart}
      />
      <FinalizeActions
        post={post}
        learning={learning}
        beforeFinalize={beforeFinalize}
        onFinalized={onFinalized}
      />
    </div>
  )
})
