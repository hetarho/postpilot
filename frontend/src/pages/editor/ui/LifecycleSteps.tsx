import { useCallback, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { isTerminal } from '@/entities/generation-job'
import {
  hasContent,
  isPublished,
  useRefreshPostImages,
  originContentMatches,
  type PostDraft,
  type PostStorylineParagraph,
} from '@/entities/post'
import type { PostContent } from '@/shared/api'
import type {
  GenerationActionsHandle,
  GenerationMode,
  StorylineActionsHandle,
} from '@/features/generate-post'
import type { BlockEditorHandle } from '@/features/edit-post-content'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { InspectWritingRequestAction } from '@/features/inspect-writing-request'
import { proseStyles } from '@/shared/ui'
import type { SaveState } from '@/features/save-draft'
import { StorylineSpace } from '@/widgets/storyline-space'
import { editorStepLabel, type EditorStep } from '../model/steps'
import type { EditorJobView } from '../model/useEditorJob'
import { EditorJobNotice } from './EditorJobNotice'
import { EditorStepDock } from './EditorStepDock'
import { EditorFinishPanel } from './EditorFinishPanel'
import { EditorGeneratePanel } from './EditorGeneratePanel'
import { EditorRefinePanel } from './EditorRefinePanel'

const STEP_PANEL_ID = 'editor-step-panel'

/** The three lifecycle panels and the ONE dock under them. The page owns the post's identity —
 *  its fields, its autosave and its mint — so nothing here may hold state that must survive a
 *  step change. */
export function LifecycleSteps({
  post,
  ownerId,
  step,
  onStepChange,
  titleField,
  memoField,
  answerFields,
  dockHeader,
  onOpenBrief,
  targetLength,
  onTitleFinalized,
  beforeStart,
  checkRequiredAnswers,
  ensureSlug,
  jobView,
  storyline,
  inspection,
}: {
  post: PostDraft
  ownerId: string
  step: EditorStep
  onStepChange: (step: EditorStep) => void
  titleField: ReactNode
  memoField: ReactNode
  /** The selected template's data fields. ①'s material, so it renders with the memo it sits
   *  under rather than anywhere the run is configured (POST-54). */
  answerFields: ReactNode
  dockHeader: ReactNode
  /** Opens the writing brief marking what a `mode` press was refused for. */
  onOpenBrief: (mode: GenerationMode) => void
  targetLength?: number
  /** Re-seeds the editor's local 가제 with what 확정 wrote into `posts.title`. */
  onTitleFinalized: (title: string) => void
  beforeStart: () => Promise<void>
  checkRequiredAnswers: () => boolean
  ensureSlug: () => Promise<string>
  /** The durable job, resolved by the page so the status region and these panels read one poll. */
  jobView: EditorJobView
  /** The storyline as the draft autosave holds it (POST-96): ② edits it through the same queue as
   *  ①'s fields, so one save carries whichever changed. */
  storyline: {
    paragraphs: PostStorylineParagraph[] | undefined
    onChange: (paragraphs: PostStorylineParagraph[]) => void
    saveState: SaveState
  }
  inspection?: { contextKey: string; sourcePending: boolean }
}) {
  const { t } = useTranslation('posts')
  const generateRef = useRef<GenerationActionsHandle>(null)
  const reviseRef = useRef<ReviseFormHandle>(null)
  const storylineRef = useRef<StorylineActionsHandle>(null)
  const contentEditorRef = useRef<BlockEditorHandle>(null)
  // A view URL is presigned and short-lived, and this screen outlives one: the post query is
  // refetched on mount and never again while the editor sits open, and a draft save deliberately
  // patches the cached image list in place rather than invalidating it. The export panel asks for
  // a fresh set when one of its photos fails to load — the only moment a dead URL costs anything,
  // because a photo that HAS painted is copied from its own pixels.
  const refreshPhotoUrls = useRefreshPostImages(post.slug)
  const { job } = jobView
  const result = hasContent(post) ? post.content : undefined
  // What export renders. The block editor's unsaved edits are newer than the server's copy, but only
  // until the server's revision moves past them — a completed revision or a landed save makes the
  // server authoritative again. Tagging the reported content with the revision it was edited from is
  // what expires it, so leaving 글 다듬기 cannot freeze export on content the post has since passed.
  const [edited, setEdited] = useState<{ revision: bigint; content: PostContent }>()
  const liveContent = edited?.revision === post.contentRevision ? edited.content : result
  // Stable except when the revision moves: `BlockEditor` reports its content from an effect that
  // depends on this callback, so a new identity every render would loop.
  const reportEdited = useCallback(
    (content: PostContent) => setEdited({ revision: post.contentRevision, content }),
    [post.contentRevision],
  )
  // 제목 · 메모 · 사진 · the voice caveat · the contact sheet. Everything that DESCRIBES the next
  // AI run left this panel for the one brief surface in the dock, so what is left is the post's
  // own material (POST-51).
  const generatePanel = (
    <EditorGeneratePanel
      post={post}
      ownerId={ownerId}
      titleField={titleField}
      memoField={memoField}
      answerFields={answerFields}
      ensureSlug={ensureSlug}
      job={job}
    />
  )
  // Read-only while a job targets the post, on a published post and while the queue reports a
  // refusal (POST-86): an edit then could only be overwritten or refused again.
  const storylineReadOnly =
    Boolean(job && !isTerminal(job)) || isPublished(post) || storyline.saveState === 'error'
  const storylineSpace =
    post.storyline && storyline.paragraphs ? (
      <StorylineSpace
        ref={storylineRef}
        post={post}
        paragraphs={storyline.paragraphs}
        onChange={storyline.onChange}
        readOnly={storylineReadOnly}
        hasContent={Boolean(result)}
        actions={{
          targetLength,
          activeJob: job,
          jobPending: jobView.isPending,
          onStarted: (id) => jobView.onStarted(id, 'refine'),
          beforeStart,
          checkRequiredAnswers,
          // Nothing to save when no block editor is mounted — a post with no content yet.
          flushContent: () => contentEditorRef.current?.flush() ?? Promise.resolve(),
          onOpenBrief,
        }}
      />
    ) : undefined
  const refinePanel = (
    <EditorRefinePanel
      post={post}
      ownerId={ownerId}
      storylineSpace={storylineSpace}
      result={result}
      editorRef={contentEditorRef}
      onContentChange={reportEdited}
      onGoGenerate={() => onStepChange('generate')}
    />
  )
  const finishPanel = (
    <EditorFinishPanel
      post={post}
      ownerId={ownerId}
      result={result}
      liveContent={liveContent}
      onPhotoUrlsStale={refreshPhotoUrls}
      onGoGenerate={() => onStepChange('generate')}
    />
  )
  const jobNotice = (
    <EditorJobNotice
      post={post}
      step={step}
      jobView={jobView}
      generateRef={generateRef}
      reviseRef={reviseRef}
      storylineRef={storylineRef}
    />
  )
  const hasJobNotice = Boolean(
    jobView.jobId && (jobView.isError || jobView.job?.status === 'failed'),
  )

  return (
    <>
      {ownerId && (
        <InspectWritingRequestAction
          target={{
            kind: 'post',
            ownerId,
            postSlug: post.slug,
            sourceRevision: String(post.inputRevision),
            resultRevision: `${post.contentRevision}:${post.contentHash ?? ''}`,
            planRevision: JSON.stringify(post.storyline),
          }}
          contextKey={JSON.stringify([inspection?.contextKey, liveContent])}
          stages={['observe', 'plan', 'write', 'write-from-storyline', 'revise']}
          blockedReason={
            inspection?.sourcePending ||
            (liveContent && result && !originContentMatches(liveContent, result))
              ? t('unsavedSource', { ns: 'requestInspection' })
              : undefined
          }
          className="my-4"
        />
      )}
      <div
        id={STEP_PANEL_ID}
        role="tabpanel"
        aria-label={editorStepLabel(step)}
        className={step === 'generate' ? undefined : proseStyles()}
      >
        {step === 'generate' ? generatePanel : step === 'refine' ? refinePanel : finishPanel}
      </div>

      {/* A finished job announces itself and takes no space. The visible success banner this
          replaced stood on EVERY step, on the bar the draft is read past, and nothing ever took
          it down: `done` is a standing STATE, not an event, so it came back on the next render
          for as long as the post's last job was that one, and its 결과 보기 only changed the step
          it was already sitting on (owner decision 2026-09-02). What it announced is not lost —
          the status change carries the user to 글 다듬기 with the draft in front of them — so what
          is kept is the part a screen reader has no other way to get.

          Mounted at all times and outside the dock's own existence test, so a bar with nothing
          visible to say is still not rendered (THEME-8). A live region inserted with its text already
          inside announces nothing, which is exactly right: this speaks on the transition to
          `done` and stays silent for a job that was already finished when the editor mounted. */}
      <p className="sr-only" role="status">
        {job?.status === 'done' ? t('editor.generationComplete') : ''}
      </p>

      <EditorStepDock
        post={post}
        ownerId={ownerId}
        step={step}
        result={Boolean(result)}
        jobView={jobView}
        jobNotice={jobNotice}
        hasJobNotice={hasJobNotice}
        dockHeader={dockHeader}
        targetLength={targetLength}
        editorRef={contentEditorRef}
        generateRef={generateRef}
        reviseRef={reviseRef}
        beforeStart={beforeStart}
        checkRequiredAnswers={checkRequiredAnswers}
        onOpenBrief={onOpenBrief}
        onTitleFinalized={onTitleFinalized}
        onStepChange={onStepChange}
      />
    </>
  )
}
