import { forwardRef } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import { isPublished, type PostDraft } from '@/entities/post'
import { ReviseForm, type ReviseFormHandle } from '@/features/edit-with-ai'
import { FinalizeActions } from '@/features/finalize-post'

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
}

/** The body of 글 다듬기's dock: ONE surface, the revision instruction with its send button, whose
 *  heading carries 확정하기 at its top-right.
 *
 *  The confirming actions used to be a second row of full-width buttons under the field, and the
 *  bar read as two competing interfaces — a conversation with the AI, and the pair that ends the
 *  step (owner decision 2026-09-02). Now the one way out, 확정하기, stands in the field's heading
 *  row and finalizes at once (POST-56), so the dock says one thing.
 *
 *  It is a WIDGET because it composes two sibling `features/*` slices — `edit-with-ai` and
 *  `finalize-post` — and a feature may not import a sibling (ARCH-13). The composition is
 *  a SLOT: the finalize control is handed to the revise form as its heading action.
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
  },
  reviseRef,
) {
  return (
    <FinalizeActions post={post} beforeFinalize={beforeFinalize} onFinalized={onFinalized}>
      {(finalize) =>
        // A published post takes no revision (POST-86): the dock keeps only the road onward to
        // 글 완성, where its address lives.
        isPublished(post) ? (
          finalize
        ) : (
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
            action={finalize}
          />
        )
      }
    </FinalizeActions>
  )
})
