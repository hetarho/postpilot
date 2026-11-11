import { forwardRef, type ReactNode } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import { isPublished, type PostDraft } from '@/entities/post'
import { ReviseForm, type ReviseFormHandle } from '@/features/edit-with-ai'
import { FinalizeActions } from '@/features/finalize-post'
import { StepBarActions } from '@/shared/ui'

interface RefineDockProps {
  ownerId: string
  post: PostDraft
  activeJob?: GenerationJob
  jobPending: boolean
  onRevisionStarted: (jobId: string) => void
  /** Flushes a pending block edit. Both surfaces take it: a finalize may never name a revision
   *  that omits an edit the user has already made. */
  beforeStart: () => Promise<void>
  beforeFinalize: () => Promise<bigint>
  /** Carries the title 확정 wrote into `posts.title`, so the editor can re-seed its 가제. */
  onFinalized: (title: string) => void
  /** The step bar's way back (a `StepBarBack`), leading the row 확정하기 fills. */
  back?: ReactNode
}

/** The body of 글 다듬기's step bar (`StepBar`, the bar every stepped screen shares): the revision
 *  instruction with its send button, and under it the row every step ends on — the way back and
 *  the step's one big action, 확정하기.
 *
 *  확정하기 used to stand in the field's heading row, inside a floating composer that looked
 *  nothing like the guide's bar a step earlier, so the two steps read as two different apps
 *  (owner decision 2026-11-11). Before that the confirming actions were a second row of
 *  full-width buttons under the field, which read as a second, competing interface (owner
 *  decision 2026-09-02). Now the field is the bar's composer and its one way out finalizes at
 *  once (POST-56), in the same place as 다음 everywhere else.
 *
 *  It is a WIDGET because it composes two sibling `features/*` slices — `edit-with-ai` and
 *  `finalize-post` — and a feature may not import a sibling (ARCH-13).
 *
 *  Each surface renders its own blockers, validation and failures above its own controls
 *  (THEME-31): the keyboard covers roughly the bottom 40% of the screen, so it may hide a control but
 *  never the reason that control is disabled. */
export const RefineDock = forwardRef<ReviseFormHandle, RefineDockProps>(function RefineDock(
  {
    ownerId,
    post,
    activeJob,
    jobPending,
    onRevisionStarted,
    beforeStart,
    beforeFinalize,
    onFinalized,
    back,
  },
  reviseRef,
) {
  return (
    <FinalizeActions post={post} beforeFinalize={beforeFinalize} onFinalized={onFinalized}>
      {(finalize) => (
        <>
          {/* A published post takes no revision (POST-86): the bar keeps only the road onward to
              글 완성, where its address lives. */}
          {!isPublished(post) && (
            <ReviseForm
              ref={reviseRef}
              ownerId={ownerId}
              postSlug={post.slug}
              voice={post.voice}
              template={post.template}
              activeJob={activeJob}
              jobPending={jobPending}
              onStarted={onRevisionStarted}
              beforeStart={beforeStart}
            />
          )}
          <StepBarActions back={back}>{finalize}</StepBarActions>
        </>
      )}
    </FinalizeActions>
  )
})
