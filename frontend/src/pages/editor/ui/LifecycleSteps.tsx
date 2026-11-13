import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate } from '@tanstack/react-router'
import { isTerminal } from '@/entities/generation-job'
import {
  hasContent,
  isPublished,
  useRefreshPostImages,
  originContentMatches,
  type ObservationEditFields,
  type PostDraft,
  type PostStorylineParagraph,
} from '@/entities/post'
import type { PostContent } from '@/shared/api'
import type { GenerationJob } from '@/entities/generation-job'
import {
  GenerationActions,
  ObserveActions,
  ReobserveButton,
  type GenerationActionsHandle,
  type GenerationMode,
  type StorylineActionsHandle,
} from '@/features/generate-post'
import { flushContentQueue, type BlockEditorHandle } from '@/features/edit-post-content'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { InspectWritingRequestAction } from '@/features/inspect-writing-request'
import { Notice, Typography, proseStyles } from '@/shared/ui'
import type { SaveState } from '@/features/save-draft'
import { ContactSheet } from '@/widgets/contact-sheet'
import { StorylineSpace } from '@/widgets/storyline-space'
import { editorStepLabel, type EditorStep } from '../model/steps'
import { resumedJobStep, type EditorJobView } from '../model/useEditorJob'
import { EditorJobNotice } from './EditorJobNotice'
import { EditorStepDock } from './EditorStepDock'
import { EditorFinishPanel, type FinishCampaign } from './EditorFinishPanel'
import { EditorRefinePanel } from './EditorRefinePanel'
import { EmptyStep } from './EmptyStep'
import { RunCard } from './RunCard'

const STEP_PANEL_ID = 'editor-step-panel'

/** The five lifecycle panels and the ONE dock under them. The page owns the post's identity —
 *  its fields, its autosave and its mint — so nothing here may hold state that must survive a
 *  step change. */
export function LifecycleSteps({
  post,
  ownerId,
  step,
  onStepChange,
  renderGuide,
  onOpenBrief,
  targetLength,
  onTitleFinalized,
  beforeStart,
  checkRequiredAnswers,
  jobView,
  storyline,
  campaign,
  observations,
  onRunCardShown,
  clipMaterial,
  inspection,
}: {
  post: PostDraft
  ownerId: string
  step: EditorStep
  onStepChange: (step: EditorStep) => void
  /** Guide: ① 생성 is the page's guided sequence (EditorGuide). The panel hands it the summary's
   *  row — the guide's 이전, then 다음 to ② 관찰 — and the last job's failure notice. */
  renderGuide: (slots: { actions: (back: ReactNode) => ReactNode; notice: ReactNode }) => ReactNode
  /** Opens the writing brief marking what a `mode` press was refused for. */
  onOpenBrief: (mode: GenerationMode) => void
  targetLength?: number
  /** Re-seeds the editor's local 가제 with what 확정 wrote into `posts.title`. */
  onTitleFinalized: (title: string) => void
  beforeStart: () => Promise<void>
  checkRequiredAnswers: () => boolean
  /** The durable job, resolved by the page so the status region and these panels read one poll. */
  jobView: EditorJobView
  /** The storyline as the draft autosave holds it (POST-96): ③ edits it through the same queue as
   *  ①'s fields, so one save carries whichever changed. */
  storyline: {
    paragraphs: PostStorylineParagraph[] | undefined
    onChange: (paragraphs: PostStorylineParagraph[]) => void
    saveState: SaveState
  }
  /** The owner's observation corrections as the draft autosave holds them (POST-126). */
  observations: {
    edits: Readonly<Record<string, ObservationEditFields>>
    onEdit: (file: string, fields: ObservationEditFields) => void
  }
  /** Whether ②'s run card is in view, so the page-top bar can step aside while it is (POST-45). */
  onRunCardShown?: (shown: boolean) => void
  /** The 체험단 parts as the draft autosave holds them; ⑤'s checklist ticks save through it. */
  campaign?: FinishCampaign
  clipMaterial?: Pick<PostDraft, 'title' | 'memo'>
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
  const navigate = useNavigate()
  // ②'s step bar row: 이전, 스토리라인 먼저 and 바로 글 쓰기, with the writing-test entry under it
  // (POST-53). The dock mounts it; the picker still opens from these actions.
  const observeActions = (back: ReactNode) => (
    <GenerationActions
      ref={generateRef}
      leading={back}
      post={post}
      targetLength={targetLength}
      activeJob={job}
      jobPending={jobView.isPending}
      onStarted={(id) => jobView.onStarted(id, 'observe')}
      beforeStart={beforeStart}
      onReviewResult={async () => {
        await beforeStart()
        await flushContentQueue(post.slug)
        await navigate({
          to: '/tests/records/$id',
          params: { id: post.pendingExperimentId },
          search: { source: post.slug },
        })
      }}
      checkRequiredAnswers={checkRequiredAnswers}
      onOpenBrief={onOpenBrief}
    />
  )
  // ①'s 확인 row: the guide's 이전, then 관찰하기 while an attachment lacks an observation — which
  // starts the observation job and carries the owner to ② — or 다음 to ② without a call
  // (POST-123). On a published post the bar says once why ① cannot change (POST-86); 다음 only moves.
  const observationStart = {
    post,
    activeJob: job,
    jobPending: jobView.isPending,
    beforeStart,
    onStarted: (id: string) => jobView.onStarted(id, 'observe'),
    onOpenBrief,
  }
  const confirmActions = (back: ReactNode) => (
    <>
      {isPublished(post) && (
        <Typography variant="label" as="p" role="status">
          {t('published.locked')}
        </Typography>
      )}
      <ObserveActions
        {...observationStart}
        back={back}
        onNext={() => onStepChange('observe')}
        observeLabel={t('guide.observe')}
        nextLabel={t('guide.next')}
      />
    </>
  )
  // A run this editor started carries the owner to the step showing its result once it is done:
  // ② after an observation, ③ after a storyline, ④ after writing (POST-125).
  const landed = useRef('')
  useEffect(() => {
    if (!job || job.status !== 'done' || !jobView.startedStep || landed.current === job.id) return
    landed.current = job.id
    onStepChange(resumedJobStep(job.kind))
  }, [job, jobView.startedStep, onStepChange])
  // ②'s observations and ③'s storyline are read-only while a job targets the post, on a published
  // post and while the queue reports a refusal (POST-86, POST-126): an edit then could only be
  // overwritten or refused again.
  const storylineReadOnly =
    Boolean(job && !isTerminal(job)) || isPublished(post) || storyline.saveState === 'error'
  const storylineActions = {
    ref: storylineRef,
    targetLength,
    activeJob: job,
    jobPending: jobView.isPending,
    onStarted: (id: string) => jobView.onStarted(id, 'storyline'),
    beforeStart,
    checkRequiredAnswers,
    // Nothing to save when no block editor is mounted — ④ is not on screen while ③'s bar is.
    flushContent: () =>
      contentEditorRef.current?.flush() ?? flushContentQueue(post.slug) ?? Promise.resolve(),
    onOpenBrief,
  }
  const running = job && !isTerminal(job) && RUN_CARD_KINDS.has(job.kind) ? job : undefined
  const observePanel = (
    <EditorObservePanel
      post={post}
      job={job}
      runCard={running && <RunCard job={running} onShown={onRunCardShown} />}
      heading={<ReobserveButton {...observationStart} />}
      edits={observations.edits}
      editable={!storylineReadOnly}
      onEdit={observations.onEdit}
    />
  )
  const storylinePanel = (
    <EditorStorylinePanel
      post={post}
      paragraphs={storyline.paragraphs}
      onChange={storyline.onChange}
      readOnly={storylineReadOnly}
      onGoObserve={() => onStepChange('observe')}
    />
  )
  const refinePanel = (
    <EditorRefinePanel
      post={post}
      ownerId={ownerId}
      result={result}
      editorRef={contentEditorRef}
      onContentChange={reportEdited}
      onGoStoryline={() => onStepChange('storyline')}
    />
  )
  const finishPanel = (
    <EditorFinishPanel
      post={post}
      ownerId={ownerId}
      result={result}
      liveContent={liveContent}
      campaign={campaign}
      clipMaterial={clipMaterial}
      beforeCreateClip={beforeStart}
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
          className="my-2 self-end"
        />
      )}
      <div
        id={STEP_PANEL_ID}
        role="tabpanel"
        aria-label={editorStepLabel(step)}
        className={
          step === 'generate' ? 'flex flex-1 flex-col' : proseStyles('animate-step-in pb-10')
        }
      >
        {step === 'generate'
          ? renderGuide({
              actions: confirmActions,
              notice: hasJobNotice ? jobNotice : undefined,
            })
          : step === 'observe'
            ? observePanel
            : step === 'storyline'
              ? storylinePanel
              : step === 'refine'
                ? refinePanel
                : finishPanel}
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
        editorRef={contentEditorRef}
        reviseRef={reviseRef}
        observeActions={observeActions}
        storylineActions={storylineActions}
        onTitleFinalized={onTitleFinalized}
        onStepChange={onStepChange}
      />
    </>
  )
}

/** The runs whose card heads ②: those started from ① or ② (POST-125). A revision belongs to ④. */
const RUN_CARD_KINDS = new Set(['observe', 'storyline', 'generate'])

/** ② 관찰: the run card while a run is going, then the attachments' observations on the contact
 *  sheet, each card corrected in place, or the line saying there is nothing to observe — its
 *  actions stay in the bar either way (POST-125, POST-126). */
function EditorObservePanel({
  post,
  job,
  runCard,
  heading,
  edits,
  editable,
  onEdit,
}: {
  post: PostDraft
  job: GenerationJob | undefined
  runCard: ReactNode
  /** 다시 관찰하기, in the heading row. */
  heading: ReactNode
  edits: Readonly<Record<string, ObservationEditFields>>
  editable: boolean
  onEdit: (file: string, fields: ObservationEditFields) => void
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      {runCard && <div className="mt-6">{runCard}</div>}
      {isPublished(post) && (
        <Notice tone="info" role="status" className="mt-4 mb-4">
          {t('published.locked')}
        </Notice>
      )}
      {post.images.length + post.videos.length > 0 ? (
        <ContactSheet
          images={post.images}
          videos={post.videos}
          observations={post.observations}
          activeJob={job}
          edits={edits}
          editable={editable}
          onEdit={onEdit}
          action={heading}
        />
      ) : (
        <Typography variant="body" as="p" className="text-content-tertiary mt-10">
          {t('editor.observeEmpty')}
        </Typography>
      )}
    </>
  )
}

/** ③ 스토리라인: the storyline as the whole step, or what it waits for with the way to ② that
 *  makes one (POST-95). */
function EditorStorylinePanel({
  post,
  paragraphs,
  onChange,
  readOnly,
  onGoObserve,
}: {
  post: PostDraft
  paragraphs: PostStorylineParagraph[] | undefined
  onChange: (paragraphs: PostStorylineParagraph[]) => void
  readOnly: boolean
  onGoObserve: () => void
}) {
  const { t } = useTranslation('posts')
  if (!post.storyline || !paragraphs)
    return (
      <EmptyStep goTo={onGoObserve} goToLabel={t('editor.goObserve')}>
        {t('editor.storylineEmpty')}
      </EmptyStep>
    )
  return (
    <>
      {isPublished(post) && (
        <Notice tone="info" role="status" className="mb-4">
          {t('published.locked')}
        </Notice>
      )}
      <StorylineSpace post={post} paragraphs={paragraphs} onChange={onChange} readOnly={readOnly} />
    </>
  )
}
