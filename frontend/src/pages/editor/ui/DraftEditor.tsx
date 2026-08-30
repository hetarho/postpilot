import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'
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
import { isEmptyProfile, useVoiceProfile } from '@/entities/voice-profile'
import type { PostContent } from '@/shared/api'
import { GenerationActions, type GenerationActionsHandle } from '@/features/generate-post'
import {
  BlockEditor,
  flushContentQueue,
  type BlockEditorHandle,
} from '@/features/edit-post-content'
import { FinalizePost } from '@/features/finalize-post'
import { SentenceFeedback } from '@/features/give-voice-feedback'
import { ReviseForm, type ReviseFormHandle } from '@/features/edit-with-ai'
import { SaveStatus, peekPendingDraft, useAutosave, type SaveState } from '@/features/save-draft'
import { StageModelSelect } from '@/features/select-model'
import {
  ActionBar,
  Badge,
  Button,
  FieldLabel,
  Notice,
  SegmentedControl,
  Textarea,
} from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { ExportPanel } from '@/widgets/export-panel'
import { VoiceWarning } from '@/widgets/voice-warning'
import { clearCaret, peekCaret, stashCaret } from '../model/editor-handoff'
import { EDITOR_STEPS, stepForStatus, type EditorStep } from '../model/steps'
import { EditorPhotos } from './EditorPhotos'

const STEP_PANEL_ID = 'editor-step-panel'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
}

/** Title + memo, autosaved, plus the post's lifecycle presented as three steps. The screen the
 *  whole input side of the product hangs off (PRD F-2).
 *
 *  The steps are PANELS, not routes: one mounted editor per slug, so a step change cannot remount
 *  the component and strand a queued save (tech/draft-autosave). Title, memo, photos and the mint
 *  plumbing therefore stay outside the panels — they are the post's identity, not one step's work. */
export function DraftEditor({ post }: DraftEditorProps) {
  const navigate = useNavigate()
  // Both fields are textareas: a Korean title fits ~14 characters across a 360px screen at
  // `text-2xl`, and a single-line input would scroll the rest of it out of a field that has no
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

  const autosave = useAutosave({
    post,
    title,
    memo,
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

  const memoField = <MemoField value={memo} onChange={setMemo} fieldRef={memoRef} />

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
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center text-sm underline"
        >
          ← 글 목록
        </Link>
        {post && <Badge>{postStatusLabel(post.status)}</Badge>}
      </div>

      {/* A post with a lifecycle navigates it first. `/posts/new` has none, so it shows no bar. */}
      {post && (
        <SegmentedControl
          value={step}
          options={EDITOR_STEPS}
          onChange={setStep}
          ariaLabel="글 단계"
          controls={STEP_PANEL_ID}
          className="mt-4"
        />
      )}

      <FieldLabel htmlFor="post-title" className="sr-only">
        제목
      </FieldLabel>
      <Textarea
        id="post-title"
        ref={titleRef}
        appearance="bare"
        rows={1}
        autoGrow
        value={title}
        // A pasted newline would otherwise be saved inside the title; the single-line input this
        // replaced dropped one for free.
        onChange={(event) => setTitle(event.target.value.replace(/\n/g, ' '))}
        onKeyDown={(event) => {
          // The title is one line even though the field wraps, so Enter moves to the memo instead
          // of typing a newline into it — but never mid-composition, where Enter is how a Hangul
          // IME commits the syllable being written.
          if (event.key !== 'Enter' || event.nativeEvent.isComposing) return
          event.preventDefault()
          memoRef.current?.focus()
        }}
        placeholder="제목"
        enterKeyHint="next"
        autoCapitalize="off"
        autoComplete="off"
        className="mt-4 text-2xl font-semibold tracking-tight"
      />

      {post ? (
        <LifecycleSteps
          post={post}
          step={step}
          onStepChange={setStep}
          memoField={memoField}
          beforeStart={autosave.flush}
          ensureSlug={autosave.ensureSlug}
          saveState={autosave.state}
        />
      ) : (
        <>
          {/* No lifecycle yet, so no step bar — just the step ① surfaces that work without a post. */}
          {memoField}
          <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />
          <EditorVoiceWarning />
          {/* A draft with no post yet has no committing action — but it is also the state where a
              failing save is most expensive, since nothing has reached the server at all. */}
          <EditorDock saveState={autosave.state} />
        </>
      )}
    </main>
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
  return (
    <>
      <FieldLabel htmlFor="post-memo" className="sr-only">
        메모
      </FieldLabel>
      {/* `text-base`, not the `body` role: the bare appearance takes its size from the caller, and
          iOS Safari zooms the whole layout — permanently, it never zooms back out — the moment a
          focused control computes under 16px (§3.1). This is the field the product exists for, so
          the zoom used to fire on every single use of the app.
          `autoGrow` with a small `rows`: at 16 rows the memo was a 364px box scrolling inside
          itself, which swallowed every vertical swipe that landed on it and left the 16px gutters
          as the only place to scroll the page (§4.4). */}
      <Textarea
        id="post-memo"
        ref={fieldRef}
        appearance="bare"
        rows={6}
        autoGrow
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder="무슨 일이 있었는지 편하게 적어 주세요"
        enterKeyHint="enter"
        className="mt-5 text-base leading-relaxed"
      />
    </>
  )
}

/** Below the memo, not above it: three wrapped lines of undismissable warning at the top of the
 *  editor pushed the writing field a fifth of a 640px screen down, for every user who has not
 *  trained a profile yet — and chrome is small, quiet and at the edges (§0). It still sits above
 *  글 생성, which is what it is a caveat about. */
function EditorVoiceWarning() {
  const { user } = useSession()
  const { profile } = useVoiceProfile(user?.id ?? '')
  if (!profile || !isEmptyProfile(profile)) return null
  return (
    <div className="mt-6">
      <VoiceWarning profile={profile} />
    </div>
  )
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
  // An untouched `/posts/new` has neither: `idle` is SaveStatus's deliberately empty state, and a
  // draft the server has not created yet carries no action — so the bar renders as a bare elevated
  // card across the bottom of the screen. Chrome with nothing to say is not chrome (§0); it comes
  // back with the first keystroke, which is also the first thing it has to report.
  if (saveState === 'idle' && !children) return null
  return (
    <>
      {/* `mt-auto` alone can resolve to zero when the page is taller than the viewport. Keep a
          real 24px spacer as well, so the step panel never touches the dock card. */}
      <div aria-hidden className="mt-auto h-6 shrink-0" />
      <ActionBar ariaLabel="저장 상태와 글 작업" className="mt-0">
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
      <p className="text-content-tertiary text-sm leading-relaxed">{children}</p>
      <Button variant="secondary" className="mt-4" onClick={goTo}>
        {goToLabel}
      </Button>
    </div>
  )
}

function LifecycleSteps({
  post,
  step,
  onStepChange,
  memoField,
  beforeStart,
  ensureSlug,
  saveState,
}: {
  post: PostDraft
  step: EditorStep
  onStepChange: (step: EditorStep) => void
  memoField: ReactNode
  beforeStart: () => Promise<void>
  ensureSlug: () => Promise<string>
  saveState: SaveState
}) {
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
  const { user } = useSession()

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

  const generatePanel = (
    <>
      {memoField}
      <EditorPhotos post={post} ensureSlug={ensureSlug} />
      <EditorVoiceWarning />
      <section aria-labelledby="generation-heading" className="mt-10">
        <h2 id="generation-heading" className="text-lg font-semibold tracking-tight">
          글 생성
        </h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-3">
          <div>
            <StageModelSelect stage="observe" optional={post.images.length === 0} />
            {post.images.length === 0 && (
              <p className="text-content-tertiary mt-1 text-xs">
                사진이 없어 관찰 모델은 필요하지 않아요.
              </p>
            )}
          </div>
          <div>
            <StageModelSelect stage="write" />
          </div>
          <div>
            <p className="text-sm font-medium">작성 A/B 후보</p>
            <Link
              to="/ai-models"
              className="text-link-fg hover:text-link-fg-hover mt-1 inline-flex min-h-11 items-center text-sm underline"
            >
              AI 모델에서 두 후보 설정
            </Link>
          </div>
        </div>
      </section>

      {post.pendingExperimentId && (!job || isTerminal(job)) && (
        <Link
          to="/ai-models/experiments/$id"
          params={{ id: post.pendingExperimentId }}
          className="bg-notice-info-bg text-notice-info-fg mt-6 flex min-h-11 items-center rounded-md px-3 py-2 text-sm font-medium"
        >
          AI 결과 확인 →
        </Link>
      )}

      {post.images.length > 0 && (
        <ContactSheet images={post.images} observations={post.observations} activeJob={job} />
      )}
    </>
  )

  const refinePanel = result ? (
    <>
      <BlockEditor
        key={`${post.slug}:${post.machineBaselineRevision}`}
        ref={contentEditorRef}
        post={post}
        onContentChange={reportEdited}
        renderSentenceAction={(text, flush) => (
          <SentenceFeedback
            postSlug={post.slug}
            text={text}
            beforeSubmit={() => flush().then(() => undefined)}
          />
        )}
      />
      <ReviseForm
        ref={reviseRef}
        postSlug={post.slug}
        activeJob={job}
        jobPending={Boolean(jobId) && !job}
        onStarted={(id) => setStarted({ id, step: 'refine' })}
        beforeStart={() =>
          (contentEditorRef.current?.flush() ?? Promise.resolve()).then(() => undefined)
        }
      />
    </>
  ) : (
    <EmptyStep goTo={() => onStepChange('generate')} goToLabel="글 생성으로 가기">
      아직 다듬을 글이 없어요. 글 생성에서 초안을 만들면 여기에서 문단별로 고칠 수 있습니다.
    </EmptyStep>
  )

  const finishPanel = result ? (
    <>
      {user && (
        <FinalizePost
          ownerId={user.id}
          post={post}
          // 확정 lives on 글 완성 and the block editor on 글 다듬기, so the ref is null here by
          // construction. The queue is keyed by slug and outlives the editor, so the pending save
          // is still waited on — and its revision used — rather than finalizing a revision that
          // omits the edit the user just made.
          beforeFinalize={() =>
            contentEditorRef.current?.flush() ??
            flushContentQueue(post.slug) ??
            Promise.resolve(post.contentRevision)
          }
        />
      )}
      <ExportPanel
        content={liveContent ?? result}
        images={post.images}
        createdAt={post.createdAt}
      />
    </>
  ) : (
    <EmptyStep goTo={() => onStepChange('generate')} goToLabel="글 생성으로 가기">
      아직 완성할 글이 없어요. 초안을 만들고 다듬으면 여기에서 확정하고 내보낼 수 있습니다.
    </EmptyStep>
  )

  // What the dock has to say about the durable job, if anything. Computed up here because it is
  // also what decides whether the bar exists at all: a job's progress and its failure must reach
  // the user on every step, while a bar holding nothing is chrome with nothing to say (§0).
  //
  // The RETRY is offered only on the step that owns the job, because that is where the control it
  // calls is mounted.
  const jobNotice = !jobId ? null : jobState.isError ? (
    <FailureNotice error="작업 상태를 확인하지 못했어요." onRetry={jobState.refetch} />
  ) : job?.status === 'failed' ? (
    <FailureNotice
      error={job.error}
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
      <span className="min-w-0">글 생성을 마쳤어요.</span>
      {result && (
        <Button
          variant="ghost"
          onClick={() => onStepChange('refine')}
          className="text-notice-success-fg shrink-0 underline"
        >
          결과 보기
        </Button>
      )}
    </Notice>
  ) : null

  return (
    <>
      <div id={STEP_PANEL_ID} role="tabpanel" aria-label={stepLabel(step)}>
        {step === 'generate' ? generatePanel : step === 'refine' ? refinePanel : finishPanel}
      </div>

      {(step === 'generate' || jobNotice || saveState === 'saving' || saveState === 'error') && (
        <EditorDock saveState={saveState}>
          {jobNotice}
          {step === 'generate' && (
            <GenerationActions
              ref={generateRef}
              post={post}
              activeJob={job}
              jobPending={Boolean(jobId) && !job}
              onStarted={(id) => setStarted({ id, step: 'generate' })}
              beforeStart={beforeStart}
            />
          )}
        </EditorDock>
      )}
    </>
  )
}

const stepLabel = (step: EditorStep) =>
  EDITOR_STEPS.find((item) => item.value === step)?.label ?? ''
