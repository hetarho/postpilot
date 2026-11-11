import type { ReactNode, RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import { flushContentQueue, type BlockEditorHandle } from '@/features/edit-post-content'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { StepBar, StepBarBack } from '@/shared/ui'
import { RefineDock } from '@/widgets/refine-dock'
import type { EditorStep } from '../model/steps'
import type { EditorJobView } from '../model/useEditorJob'

/** ②'s step bar — 다듬기 with the AI, then ← and 확정하기 — and ③'s failure notice when the job has
 *  something to report, both on the same `StepBar` as 글 생성's guided flow (EditorGuide), so this
 *  docks only on the two later steps. There is exactly ONE bar in this scroller either way.
 *
 *  It is mounted as the LAST child of `main`: a `sticky bottom-*` box is only pinned while its
 *  containing block still extends below it, so anywhere earlier in the flow it would scroll away
 *  with the section it sat in. A bar holding neither a control nor a refusal is not chrome
 *  (THEME-8), so ③ has one only while a failure has something to say. */
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
  const { t } = useTranslation('posts')
  const job: GenerationJob | undefined = jobView.job
  if (step === 'generate') return null
  if (!((step === 'refine' && Boolean(result)) || hasJobNotice)) return null
  return (
    <StepBar ariaLabel={t('editor.actionAria')}>
      {jobNotice}
      {step === 'refine' && result && (
        <RefineDock
          ref={reviseRef}
          back={
            <StepBarBack
              label={t('guide.back')}
              onClick={() => {
                onStepChange('generate')
                window.scrollTo(0, 0)
              }}
            />
          }
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
    </StepBar>
  )
}
