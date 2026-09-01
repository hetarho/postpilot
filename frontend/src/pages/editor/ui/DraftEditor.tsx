import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import {
  getPostQueryKey,
  hasContent,
  listPostsQueryKey,
  postStatusLabel,
  type PostDraft,
} from '@/entities/post'
import { useSession } from '@/entities/session'
import {
  useVoiceProfile,
  voiceContentLanguageMismatch,
  voiceContentLanguageMismatchReason,
  type VoiceRef,
} from '@/entities/voice'
import type { ContentLanguage, PostContent } from '@/shared/api'
import { GenerationActions, type GenerationActionsHandle } from '@/features/generate-post'
import {
  BlockEditor,
  flushContentQueue,
  type BlockEditorHandle,
} from '@/features/edit-post-content'
import { VoiceLearningPanel, useVoiceLearning } from '@/features/finalize-post'
import { SentenceFeedback } from '@/features/give-voice-feedback'
import { type ReviseFormHandle } from '@/features/edit-with-ai'
import { SaveStatus, peekPendingDraft, useAutosave, type SaveState } from '@/features/save-draft'
import { reassignmentBlocker } from '@/features/select-post-voice'
import {
  ActionBar,
  Badge,
  Button,
  FieldLabel,
  Notice,
  SegmentedControl,
  Textarea,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { ExportPanel } from '@/widgets/export-panel'
import { GenerationBrief } from '@/widgets/generation-brief'
import { PublishPanel } from '@/widgets/publish-panel'
import { RefineDock } from '@/widgets/refine-dock'
import { DeletedVoiceWarning, VoiceWarning } from '@/widgets/voice-warning'
import { clearCaret, peekCaret, stashCaret } from '../model/editor-handoff'
import { activeLocale } from '@/shared/lib'
import { editorStepLabel, editorSteps, stepForStatus, type EditorStep } from '../model/steps'
import { EditorPhotos } from './EditorPhotos'

const STEP_PANEL_ID = 'editor-step-panel'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
  /** The voice a draft with no post yet starts in — the account's default, resolved by the
   *  route before this mounts, so the first save always carries a concrete id
   *  (spec/policy/posts.md). Ignored for an existing post, whose voice is its own. */
  defaultVoiceId?: string
}

/** Title + memo, autosaved, plus the post's lifecycle presented as three steps. The screen the
 *  whole input side of the product hangs off (PRD F-2).
 *
 *  The steps are PANELS, not routes: one mounted editor per slug, so a step change cannot remount
 *  the component and strand a queued save (tech/draft-autosave). Title, memo, photos and the mint
 *  plumbing therefore stay outside the panels — they are the post's identity, not one step's work. */
export function DraftEditor({ post, defaultVoiceId = '' }: DraftEditorProps) {
  const { t } = useTranslation('posts')
  const navigate = useNavigate()
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  // Both fields are textareas: a Korean title fits ~14 characters across a 360px screen at the
  // display size, and a single-line input would scroll the rest of it out of a field that has no
  // well to show it scrolled (design-language §0 — the title is one of the largest things on the
  // screen, so it wraps instead).
  const titleRef = useRef<HTMLTextAreaElement>(null)
  const memoRef = useRef<HTMLTextAreaElement>(null)

  // Text still queued for this post outranks what the server reported: it is what the
  // previous editor was in the middle of saving when the mint moved the URL, so it is
  // newer by exactly the characters typed during that round trip.
  const opening = post
    ? (peekPendingDraft(post.slug) ?? { title: post.title, memo: post.memo })
    : { title: '', memo: '' }
  const [title, setTitle] = useState(opening.title)
  const [memo, setMemo] = useState(opening.memo)

  // Read, not consumed — a component body may run more than once per mount.
  const caret = post ? peekCaret(post.slug) : undefined

  // A draft with no post yet holds its own choice; an existing post's voice is the server's,
  // and changes only through a confirmed reassignment (see `useAutosave.reassign`).
  const [newVoiceId, setNewVoiceId] = useState(defaultVoiceId)
  const voiceId = post ? post.voice.id : newVoiceId
  const voice: VoiceRef = post?.voice ?? {
    id: newVoiceId,
    name: '',
    deleted: false,
    sourceLanguage: undefined,
  }

  // The same shape for the 용도, with 없음 ('') as the default the server never overrides.
  const [newPurposeId, setNewPurposeId] = useState('')
  const purposeId = post ? post.purpose.id : newPurposeId
  // Snapshot once when /posts/new opens. Later interface-locale switches are presentation-only;
  // the explicit selector is the sole way this draft's target changes.
  const [newTargetLanguage, setNewTargetLanguage] = useState<ContentLanguage>(() => activeLocale())
  const targetLanguage = post?.targetLanguage ?? newTargetLanguage

  const autosave = useAutosave({
    post,
    title,
    memo,
    voiceId,
    purposeId,
    targetLanguage,
    onMinted: (slug) => {
      // Read off the live DOM, so this is the caret as it is now rather than as it was
      // when the save left.
      const focused = document.activeElement
      const field =
        focused === titleRef.current ? 'title' : focused === memoRef.current ? 'memo' : undefined
      if (field) {
        const element = field === 'title' ? titleRef.current : memoRef.current
        stashCaret({
          slug,
          field,
          selectionStart: element?.selectionStart ?? 0,
          selectionEnd: element?.selectionEnd ?? 0,
        })
      }
      // `replace`, so the back button goes to the list rather than to /posts/new — which
      // would open a second empty draft.
      void navigate({ to: '/posts/$slug', params: { slug }, replace: true })
    },
  })

  // The step lives here, above the fields, because the bar that switches it is the first thing
  // on the screen — the post's lifecycle is what you navigate before you read anything else.
  const [step, setStep] = useState<EditorStep>(() => stepForStatus(post?.status ?? ''))
  const followedStatus = useRef(post?.status ?? '')
  useEffect(() => {
    const status = post?.status ?? ''
    if (followedStatus.current === status) return
    followedStatus.current = status
    // Also the "generation finished" handoff: the draft lands in 글 다듬기 and the user is taken
    // there once, rather than being scrolled inside 글 생성 and then moved again.
    setStep(stepForStatus(status))
  }, [post?.status])

  useEffect(() => {
    if (!caret) return
    clearCaret()
    const element = caret.field === 'title' ? titleRef.current : memoRef.current
    if (!element) return
    element.focus()
    element.setSelectionRange(caret.selectionStart, caret.selectionEnd)
  }, [caret])

  // Lifted out of `GenerationActions`: the brief widget SETS the target length and the generate
  // action SENDS it, and the two now live on different layers, so the value has to be owned by
  // the one screen they both hang off.
  // It is adjusted DURING RENDER rather than from an effect — an effect would paint one frame
  // carrying the value the server has already moved past.
  const [targetLength, setTargetLength] = useState(post?.targetLength)
  const [serverTargetLength, setServerTargetLength] = useState(post?.targetLength)
  if (serverTargetLength !== post?.targetLength) {
    setServerTargetLength(post?.targetLength)
    setTargetLength(post?.targetLength)
  }

  const titleField = (
    <TitleField value={title} onChange={setTitle} fieldRef={titleRef} nextRef={memoRef} />
  )
  const memoField = <MemoField value={memo} onChange={setMemo} fieldRef={memoRef} />

  // Everything the next AI run is given, in one surface. Every callback goes through the autosave
  // queue for an existing post, and through local state for a draft the server has not created.
  const brief = (
    <GenerationBrief
      ownerId={ownerId}
      voiceId={voiceId}
      currentVoice={post?.voice}
      voiceBlocked={post ? reassignmentBlocker(post) : ''}
      confirmVoiceChange={Boolean(post)}
      onVoiceSelect={post ? autosave.reassign : setNewVoiceId}
      purposeId={purposeId}
      currentPurpose={post?.purpose}
      jobRunning={Boolean(post?.activeJob && !isTerminal(post.activeJob))}
      onPurposeSelect={post ? autosave.assignPurpose : setNewPurposeId}
      targetLanguage={targetLanguage}
      contentLanguage={post?.contentLanguage}
      frozenLanguage={
        post?.activeJob && !isTerminal(post.activeJob) ? post.activeJob.targetLanguage : undefined
      }
      onTargetLanguageSelect={post ? autosave.assignTargetLanguage : setNewTargetLanguage}
      photoCount={post?.images.length ?? 0}
      targetLength={
        post
          ? {
              slug: post.slug,
              value: targetLength,
              // The length is the one field in the brief a running job has already frozen; the
              // others are still worth changing for the NEXT run, which is why only this one greys.
              disabled: Boolean(post.activeJob && !isTerminal(post.activeJob)),
              onSaved: setTargetLength,
            }
          : undefined
      }
    />
  )

  // `flex-1 flex-col` here plus `mt-auto` on the dock is what puts the bar at the BOTTOM of a
  // short draft: `sticky` can only pull an element up toward the scrollport edge, never push one
  // down, so without it a new draft renders its dock mid-page with dead space beneath.
  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col px-4 py-6 sm:px-6">
      <div className="flex items-center justify-between gap-3">
        {/* Underlined: `link-fg` resolves to `content-secondary`, so at rest this was pixel-identical
            to ordinary copy and the only thing marking it as the way out was a `hover:` colour no
            touchscreen ever matches (§6). */}
        <Link
          to="/posts"
          className={typographyStyles({
            variant: 'label',
            className:
              'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center underline',
          })}
        >
          {t('editor.backToList')}
        </Link>
        {post && <Badge>{postStatusLabel(post.status)}</Badge>}
      </div>

      {/* A post with a lifecycle navigates it first. `/posts/new` has none, so it shows no bar. */}
      {post && (
        <SegmentedControl
          value={step}
          options={editorSteps()}
          onChange={setStep}
          ariaLabel={t('editor.stepAria')}
          controls={STEP_PANEL_ID}
          className="mt-4"
        />
      )}

      {post ? (
        <LifecycleSteps
          post={post}
          ownerId={ownerId}
          step={step}
          onStepChange={setStep}
          titleField={titleField}
          memoField={memoField}
          brief={brief}
          targetLength={targetLength}
          onTitleFinalized={setTitle}
          beforeStart={autosave.flush}
          ensureSlug={autosave.ensureSlug}
          saveState={autosave.state}
        />
      ) : (
        <>
          {/* No lifecycle yet, so no step bar — just the step ① surfaces that work without a post. */}
          {titleField}
          {memoField}
          <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />
          <EditorVoiceWarning ownerId={ownerId} voice={voice} />
          {/* A draft with no post yet has no committing action, but the brief is still where its
              말투 is chosen — and that choice has to be reachable before the first word is typed. */}
          <EditorDock saveState={autosave.state}>{brief}</EditorDock>
        </>
      )}
    </main>
  )
}

/** The 가제 belongs to 글 생성 alone (policy/posts.md). From 글 다듬기 on there is exactly one
 *  title on the screen and it is `content.title`, edited through the block editor's header — two
 *  title fields side by side is a question the user cannot answer.
 *
 *  A textarea, not an input: a Korean title fits ~14 characters across a 360px screen at the
 *  display size, and a single-line input would scroll the rest of it out of a field that has no
 *  well to show it scrolled (§0 — the title is one of the largest things on the screen, so it
 *  wraps instead). Its value and its autosave stay in `DraftEditor`, so unmounting it on another
 *  step cannot strand a queued save. */
function TitleField({
  value,
  onChange,
  fieldRef,
  nextRef,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
  nextRef: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-title" className="sr-only">
        {t('editor.title')}
      </FieldLabel>
      {/* The visible title remains an editable field, so this mirrors its current value into the
          document outline without replacing the field or creating a second tab stop. */}
      <Typography variant="display" className="sr-only">
        {value.trim() || t('editor.titlePlaceholder')}
      </Typography>
      <Textarea
        id="post-title"
        ref={fieldRef}
        appearance="bare"
        rows={1}
        autoGrow
        value={value}
        // A pasted newline would otherwise be saved inside the title; the single-line input this
        // replaced dropped one for free.
        onChange={(event) => onChange(event.target.value.replace(/\n/g, ' '))}
        onKeyDown={(event) => {
          // The title is one line even though the field wraps, so Enter moves to the memo instead
          // of typing a newline into it — but never mid-composition, where Enter is how a Hangul
          // IME commits the syllable being written.
          if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
          event.preventDefault()
          nextRef.current?.focus()
        }}
        placeholder={t('editor.titlePlaceholder')}
        enterKeyHint="next"
        autoCapitalize="off"
        autoComplete="off"
        // The bare editor's caller owns the field's type (§7): the title wears the display role —
        // the post title is the largest thing on the screen (§0).
        className={typographyStyles({ variant: 'display', className: 'mt-4' })}
      />
    </>
  )
}

/** The memo is the post's own words, and it is what 글 생성 works from — so it is rendered by
 *  that step while its value and its autosave stay in `DraftEditor`. Unmounting the field on
 *  another step cannot strand a queued save: the queue is keyed by slug and lives above it. */
function MemoField({
  value,
  onChange,
  fieldRef,
}: {
  value: string
  onChange: (value: string) => void
  fieldRef: RefObject<HTMLTextAreaElement | null>
}) {
  const { t } = useTranslation('posts')
  return (
    <>
      <FieldLabel htmlFor="post-memo" className="sr-only">
        {t('editor.memo')}
      </FieldLabel>
      {/* `autoGrow` with a small `rows`: at 16 rows the memo was a 364px box scrolling inside
          itself, which swallowed every vertical swipe that landed on it and left the 16px gutters
          as the only place to scroll the page (§4.4). The well appearance owns the §3.1 input
          size, so the focused field remains at least 16px on a phone. */}
      <Textarea
        id="post-memo"
        ref={fieldRef}
        appearance="well"
        rows={6}
        autoGrow
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t('editor.memoPlaceholder')}
        enterKeyHint="enter"
        className="mt-5"
      />
    </>
  )
}

/** Below the memo, not above it: three wrapped lines of undismissable warning at the top of the
 *  editor pushed the writing field a fifth of a 640px screen down, for every user who has not
 *  trained a profile yet — and chrome is small, quiet and at the edges (§0). It still sits above
 *  글 생성, which is what it is a caveat about.
 *
 *  A deleted voice is the other caveat, and the louder one: nothing AI will run until it is
 *  restored or the post is moved. Its profile is not read — a tombstone's emptiness is not the
 *  point. */
function EditorVoiceWarning({ ownerId, voice }: { ownerId: string; voice: VoiceRef }) {
  if (voice.deleted) {
    return (
      <div className="mt-6">
        <DeletedVoiceWarning ownerId={ownerId} voice={voice} />
      </div>
    )
  }
  if (!voice.id) return null
  return <EmptyProfileWarning ownerId={ownerId} voiceId={voice.id} />
}

function EmptyProfileWarning({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { profile } = useVoiceProfile(ownerId, voiceId)
  const warning = <VoiceWarning profile={profile} voiceId={voiceId} />
  if (!profile) return null
  return <div className="mt-6 empty:hidden">{warning}</div>
}

/** The editor's docked bar. Both of the things a phone could not reach live here: the one
 *  committing action, which sat in normal flow roughly 1,000px down a 4,000px page, and the save
 *  state, which was only ever visible in the top 3% of that scroll — on a screen that has no save
 *  button, so a failing autosave was silent everywhere else (§4.3).
 *
 *  It is mounted as the LAST child of `main` on purpose: a `sticky bottom-*` box is only pinned
 *  while its containing block still extends below it, so anywhere earlier in the flow it would
 *  scroll away with the section it sat in. */
function EditorDock({ saveState, children }: { saveState: SaveState; children?: ReactNode }) {
  const { t } = useTranslation('posts')
  // A bar with nothing to say is not chrome (§0). `idle` is SaveStatus's deliberately empty state,
  // so a dock holding neither a save line nor a control is not rendered at all. Every step that
  // carries an action passes one, which is why 글 다듬기's dock is now always present.
  if (saveState === 'idle' && !children) return null
  return (
    <>
      {/* `mt-auto` alone can resolve to zero when the page is taller than the viewport. Keep a
          real 24px spacer as well, so the step panel never touches the dock card. */}
      <div aria-hidden className="mt-auto h-6 shrink-0" />
      <ActionBar ariaLabel={t('editor.actionAria')} className="mt-0">
        {/* A fixed gap even while the save line is empty, so the bar does not change height — and
            move the button out from under the thumb — as the state changes (§6). */}
        <div className="flex flex-col gap-2">
          <SaveStatus state={saveState} />
          {children}
        </div>
      </ActionBar>
    </>
  )
}

/** A step the post has not reached yet. Never a disabled tab: the point of showing all three from
 *  the first screen is that the shape of the flow is visible, so an empty step says what it is
 *  waiting for and offers the way to the step that produces it. */
function EmptyStep({
  children,
  goTo,
  goToLabel,
}: {
  children: ReactNode
  goTo: () => void
  goToLabel: string
}) {
  return (
    <div className="mt-10">
      <Typography variant="body" className="text-content-tertiary">
        {children}
      </Typography>
      <Button variant="secondary" className="mt-4" onClick={goTo}>
        {goToLabel}
      </Button>
    </div>
  )
}

function LifecycleSteps({
  post,
  ownerId,
  step,
  onStepChange,
  titleField,
  memoField,
  brief,
  targetLength,
  onTitleFinalized,
  beforeStart,
  ensureSlug,
  saveState,
}: {
  post: PostDraft
  ownerId: string
  step: EditorStep
  onStepChange: (step: EditorStep) => void
  titleField: ReactNode
  memoField: ReactNode
  brief: ReactNode
  targetLength?: number
  /** Re-seeds the editor's local 가제 with what 확정 wrote into `posts.title`. */
  onTitleFinalized: (title: string) => void
  beforeStart: () => Promise<void>
  ensureSlug: () => Promise<string>
  saveState: SaveState
}) {
  const { t } = useTranslation('posts')
  const navigate = useNavigate()
  const transport = useTransport()
  const queryClient = useQueryClient()
  const generateRef = useRef<GenerationActionsHandle>(null)
  const reviseRef = useRef<ReviseFormHandle>(null)
  const contentEditorRef = useRef<BlockEditorHandle>(null)
  // The step is recorded with the job because the retry lives there: `generateRef` is mounted only
  // on 글 생성 and `reviseRef` only on 글 다듬기, so reporting a failure on the other step would
  // offer a retry that reaches nothing.
  const [started, setStarted] = useState<{ id: string; step: EditorStep } | null>(null)
  // Above both panels on purpose: 확정하고 말투 학습 lives at the end of 글 다듬기 and every
  // learning outcome is reported on 글 완성, so the run has to outlive the step change that the
  // finalize itself causes.
  const learning = useVoiceLearning(ownerId, post)
  const { user } = useSession()
  const languageMismatch = voiceContentLanguageMismatch(
    post.contentLanguage,
    post.voice.sourceLanguage,
  )
  const jobId = started?.id || post.activeJob?.id || ''
  const postKey = useMemo(() => getPostQueryKey(transport, post.slug), [post.slug, transport])
  const postsKey = useMemo(() => listPostsQueryKey(transport), [transport])
  const invalidateOnDone = useMemo(() => [postKey, postsKey], [postKey, postsKey])
  const jobState = useJob(jobId, invalidateOnDone)
  const job = jobState.job ?? (post.activeJob?.id === jobId ? post.activeJob : undefined)
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
  // A resumed job has no recorded step, so its kind says which one owns it.
  const jobStep: EditorStep =
    started?.id === jobId ? started.step : job?.kind === 'revise' ? 'refine' : 'generate'

  // Observations are persisted batch-by-batch on the post, not on the job. Refresh that
  // read model whenever observe progress changes so the contact sheet fills while the
  // durable job is still running.
  const refreshedSnapshot = useRef('')
  useEffect(() => {
    if (!job || isTerminal(job)) return
    const snapshot = `${job.id}:${job.updatedAt}:${job.progressDone}:${job.progressTotal}`
    if (refreshedSnapshot.current === snapshot) return
    refreshedSnapshot.current = snapshot
    void queryClient.invalidateQueries({ queryKey: postKey })
  }, [job, postKey, queryClient])

  // 제목 · 메모 · 사진 · the voice caveat · the contact sheet. Everything that DESCRIBES the next
  // AI run left this panel for the one brief surface in the dock, so what is left is the post's
  // own material (change 12).
  const generatePanel = (
    <>
      {titleField}
      {memoField}
      <EditorPhotos post={post} ensureSlug={ensureSlug} />
      <EditorVoiceWarning ownerId={ownerId} voice={post.voice} />

      {post.pendingExperimentId && (!job || isTerminal(job)) && (
        <Link
          to="/ai-models/experiments/$id"
          params={{ id: post.pendingExperimentId }}
          className={typographyStyles({
            variant: 'label',
            className:
              'bg-notice-info-bg text-notice-info-fg mt-6 flex min-h-11 items-center rounded-md px-3 py-2',
          })}
        >
          {t('editor.reviewAiResult')}
        </Link>
      )}

      {post.images.length > 0 && (
        <ContactSheet images={post.images} observations={post.observations} activeJob={job} />
      )}
    </>
  )

  const refinePanel = result ? (
    <>
      {languageMismatch && (
        <Notice tone="warning" role="status" className="mb-4">
          {voiceContentLanguageMismatchReason()}
        </Notice>
      )}
      <BlockEditor
        key={`${post.slug}:${post.machineBaselineRevision}`}
        ref={contentEditorRef}
        post={post}
        onContentChange={reportEdited}
        // Feedback is evidence for the voice, and a deleted voice cannot take any; the server
        // refuses it, so the control is not offered rather than offered to fail.
        renderSentenceAction={
          post.voice.deleted || languageMismatch
            ? undefined
            : (text, flush) => (
                <SentenceFeedback
                  postSlug={post.slug}
                  text={text}
                  beforeSubmit={() => flush().then(() => undefined)}
                />
              )
        }
      />
    </>
  ) : (
    <EmptyStep goTo={() => onStepChange('generate')} goToLabel={t('editor.goGenerate')}>
      {t('editor.refineEmpty')}
    </EmptyStep>
  )

  const finishPanel = result ? (
    <>
      {ownerId && (
        <VoiceLearningPanel learning={learning} onBackToRefine={() => onStepChange('refine')} />
      )}
      {post.contentLanguage ? (
        <ExportPanel
          content={liveContent ?? result}
          images={post.images}
          createdAt={post.createdAt}
          contentLanguage={post.contentLanguage}
        />
      ) : (
        <Notice tone="danger" role="alert" className="mt-10">
          {t('export.languageMissing')}
        </Notice>
      )}
      {/* Publishing is the operator's surface (plan 17): every PublishingService procedure is
          refused to another tier, so the panel would show a pair-and-publish flow that cannot
          complete. The server stays authoritative — this only keeps the promise off the screen. */}
      {user?.plan === 'master' && (
        <PublishPanel
          ownerId={ownerId}
          postSlug={post.slug}
          contentRevision={post.contentRevision}
          finalizedRevision={post.finalizedRevision}
          status={post.status}
          beforePublish={() =>
            contentEditorRef.current?.flush() ??
            flushContentQueue(post.slug) ??
            Promise.resolve(post.contentRevision)
          }
        />
      )}
    </>
  ) : (
    <EmptyStep goTo={() => onStepChange('generate')} goToLabel={t('editor.goGenerate')}>
      {t('editor.finishEmpty')}
    </EmptyStep>
  )

  // What the dock has to say about the durable job, if anything. Computed up here because it is
  // also what decides whether the bar exists at all: a job's progress and its failure must reach
  // the user on every step, while a bar holding nothing is chrome with nothing to say (§0).
  //
  // The RETRY is offered only on the step that owns the job, because that is where the control it
  // calls is mounted.
  const jobNotice = !jobId ? null : jobState.isError ? (
    <FailureNotice message={t('editor.jobLoadFailed')} onRetry={jobState.refetch} />
  ) : job?.status === 'failed' ? (
    <FailureNotice
      failure={job.failure}
      onRetry={
        step !== jobStep || (job.kind === 'revise' && started?.id !== job.id)
          ? undefined
          : () =>
              job.kind === 'revise'
                ? reviseRef.current?.start()
                : job.kind === 'model_experiment'
                  ? post.pendingExperimentId
                    ? void navigate({
                        to: '/ai-models/experiments/$id',
                        params: { id: post.pendingExperimentId },
                      })
                    : undefined
                  : generateRef.current?.startGeneration()
      }
    />
  ) : job && !isTerminal(job) ? (
    <ProgressLine job={job} />
  ) : job?.status === 'done' ? (
    <Notice tone="success" role="status">
      <span className="min-w-0">{t('editor.generationComplete')}</span>
      {result && (
        <Button
          variant="ghost"
          onClick={() => onStepChange('refine')}
          className="text-notice-success-fg shrink-0 underline"
        >
          {t('editor.viewResult')}
        </Button>
      )}
    </Notice>
  ) : null

  return (
    <>
      <div id={STEP_PANEL_ID} role="tabpanel" aria-label={editorStepLabel(step)}>
        {step === 'generate' ? generatePanel : step === 'refine' ? refinePanel : finishPanel}
      </div>

      {/* ① and ② both always dock: 생성 ends the first step and 확정 ends the second, and the
          draft between them is routinely thousands of pixels tall (§4.3). ③ still docks only when
          the job has something to report. There is exactly ONE ActionBar in this scroller either
          way — the revision and finalize sections no longer exist in the panel. */}
      {(step === 'generate' ||
        (step === 'refine' && Boolean(result)) ||
        jobNotice ||
        saveState === 'saving' ||
        saveState === 'error') && (
        <EditorDock saveState={saveState}>
          {jobNotice}
          {step === 'generate' && (
            <GenerationActions
              ref={generateRef}
              post={post}
              targetLength={targetLength}
              activeJob={job}
              jobPending={Boolean(jobId) && !job}
              onStarted={(id) => setStarted({ id, step: 'generate' })}
              beforeStart={beforeStart}
              brief={brief}
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
              jobPending={Boolean(jobId) && !job}
              onRevisionStarted={(id) => setStarted({ id, step: 'refine' })}
              // The block editor is mounted in the panel above, so the flush is a live ref; the
              // queue's fallback covers the beat between a step change and its unmount. A finalize
              // may never name a revision that omits an edit the user has already made.
              beforeStart={() =>
                (contentEditorRef.current?.flush() ?? Promise.resolve()).then(() => undefined)
              }
              beforeFinalize={() =>
                contentEditorRef.current?.flush() ??
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
      )}
    </>
  )
}
