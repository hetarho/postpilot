import type { ReactNode, RefObject } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { flushContentQueue, type BlockEditorHandle } from '@/features/edit-post-content'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { RefineDock } from '@/widgets/refine-dock'
import type { EditorStep } from '../model/steps'
import type { EditorJobView } from '../model/useEditorJob'
import { EditorDock } from './EditorDock'

/** ②'s bottom composer — 다듬기 with the AI, and 확정하기 — and ③'s failure notice when the job has
 *  something to report. 글 생성's bar is the guided flow's own (EditorGuide), so this docks only on
 *  the two later steps. There is exactly ONE bar in this scroller either way. */
export function EditorStepDock({
  post,
  ownerId,
  step,
  result,
  jobView,
  jobNotice,
  hasJobNotice,
  editorRef,
  reviseRef,
  onTitleFinalized,
  onStepChange,
}: {
  post: PostDraft
  ownerId: string
  step: EditorStep
  /** Whether a run has produced content; ②'s dock is about committing it. */
  result: boolean
  jobView: EditorJobView
  jobNotice: ReactNode
  hasJobNotice: boolean
  editorRef: RefObject<BlockEditorHandle | null>
  reviseRef: RefObject<ReviseFormHandle | null>
  onTitleFinalized: (title: string) => void
  onStepChange: (step: EditorStep) => void
}) {
  const job: GenerationJob | undefined = jobView.job
  if (step === 'generate') return null
  if (!((step === 'refine' && Boolean(result)) || hasJobNotice)) return null
  return (
    <EditorDock>
      {jobNotice}
      {step === 'refine' && result && (
        <RefineDock
          ref={reviseRef}
          ownerId={ownerId}
          post={post}
          activeJob={job}
          jobPending={jobView.isPending}
          onRevisionStarted={(id) => jobView.onStarted(id, 'refine')}
          // The block editor is mounted in the panel above, so the flush is a live ref; the
          // queue's fallback covers the beat between a step change and its unmount. A finalize
          // may never name a revision that omits an edit the user has already made.
          beforeStart={() =>
            (editorRef.current?.flush() ?? Promise.resolve()).then(() => undefined)
          }
          beforeFinalize={() =>
            editorRef.current?.flush() ??
            flushContentQueue(post.slug) ??
            Promise.resolve(post.contentRevision)
          }
          onFinalized={(finalizedTitle) => {
            // 확정 copies the AI title into `posts.title` on the server. The editor still holds
            // the 가제 in state and `useAutosave` sends it on every save, so without this the
            // next keystroke would write the placeholder straight back over it.
            onTitleFinalized(finalizedTitle)
            onStepChange('finish')
          }}
        />
      )}
    </EditorDock>
  )
}
