import type { ReactNode, RefObject } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import type { PostDraft } from '@/entities/post'
import {
  GenerationActions,
  type GenerationActionsHandle,
  type GenerationMode,
} from '@/features/generate-post'
import { flushContentQueue, type BlockEditorHandle } from '@/features/edit-post-content'
import type { useVoiceLearning } from '@/features/finalize-post'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { RefineDock } from '@/widgets/refine-dock'
import type { EditorStep } from '../model/steps'
import type { EditorJobView } from '../model/useEditorJob'
import { EditorDock } from './EditorDock'

/** ① and ② both always dock: 생성 ends the first step and 확정 ends the second, and the draft
 *  between them is routinely thousands of pixels tall (§4.3). ③ docks only when the job has
 *  something to report. There is exactly ONE ActionBar in this scroller either way. */
export function EditorStepDock({
  post,
  ownerId,
  step,
  result,
  jobView,
  jobNotice,
  hasJobNotice,
  dockHeader,
  targetLength,
  languageMismatch,
  learning,
  editorRef,
  generateRef,
  reviseRef,
  beforeStart,
  onOpenBrief,
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
  dockHeader: ReactNode
  targetLength?: number
  languageMismatch: boolean
  learning: ReturnType<typeof useVoiceLearning>
  editorRef: RefObject<BlockEditorHandle | null>
  generateRef: RefObject<GenerationActionsHandle | null>
  reviseRef: RefObject<ReviseFormHandle | null>
  beforeStart: () => Promise<void>
  /** Opens the writing brief marking what a `mode` press was refused for. */
  onOpenBrief: (mode: GenerationMode) => void
  onTitleFinalized: (title: string) => void
  onStepChange: (step: EditorStep) => void
}) {
  const job: GenerationJob | undefined = jobView.job
  if (!(step === 'generate' || (step === 'refine' && Boolean(result)) || hasJobNotice)) return null
  return (
    <EditorDock header={step === 'generate' ? dockHeader : undefined}>
      {jobNotice}
      {step === 'generate' && (
        <GenerationActions
          ref={generateRef}
          post={post}
          targetLength={targetLength}
          activeJob={job}
          jobPending={jobView.isPending}
          onStarted={(id) => jobView.onStarted(id, 'generate')}
          beforeStart={beforeStart}
          onOpenBrief={onOpenBrief}
        />
      )}
      {step === 'refine' && result && (
        <RefineDock
          ref={reviseRef}
          ownerId={ownerId}
          post={post}
          ruleLanguageMismatch={languageMismatch}
          learning={learning}
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
