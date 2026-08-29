import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import {
  BlockList,
  getPostQueryKey,
  hasContent,
  listPostsQueryKey,
  postStatusLabel,
  type PostDraft,
} from '@/entities/post'
import { useSession } from '@/entities/session'
import { isEmptyProfile, useVoiceProfile } from '@/entities/voice-profile'
import { GenerateButton, type GenerateButtonHandle } from '@/features/generate-post'
import { ReviseForm, type ReviseFormHandle } from '@/features/edit-with-ai'
import { SaveStatus, peekPendingDraft, useAutosave, type SaveState } from '@/features/save-draft'
import { StageModelSelect } from '@/features/select-model'
import { ActionBar, Badge, Button, FieldLabel, Notice, Textarea } from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
import { ExportPanel } from '@/widgets/export-panel'
import { VoiceWarning } from '@/widgets/voice-warning'
import { clearCaret, peekCaret, stashCaret } from '../model/editor-handoff'
import { EditorPhotos } from './EditorPhotos'

interface DraftEditorProps {
  /** The saved post being edited, or undefined for a draft the server has not created
   *  yet (`/posts/new`). */
  post?: PostDraft
}

/** Title + memo, autosaved. The screen the whole input side of the product hangs off
 *  (PRD F-2); photos, generation, revision and export all extend it in later plans. */
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

  useEffect(() => {
    if (!caret) return
    clearCaret()
    const element = caret.field === 'title' ? titleRef.current : memoRef.current
    if (!element) return
    element.focus()
    element.setSelectionRange(caret.selectionStart, caret.selectionEnd)
  }, [caret])

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-6 sm:px-6">
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
        ref={memoRef}
        appearance="bare"
        rows={6}
        autoGrow
        value={memo}
        onChange={(event) => setMemo(event.target.value)}
        placeholder="무슨 일이 있었는지 편하게 적어 주세요"
        enterKeyHint="enter"
        className="mt-5 text-base leading-relaxed"
      />

      <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />

      <EditorVoiceWarning />

      {post ? (
        <GenerationSection post={post} beforeStart={autosave.flush} saveState={autosave.state} />
      ) : (
        // A draft with no post yet has no committing action — but it is also the state where a
        // failing save is most expensive, since nothing has reached the server at all.
        <EditorDock saveState={autosave.state} />
      )}
    </main>
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
  return (
    <ActionBar ariaLabel="저장 상태와 글 작업">
      {/* A fixed gap even while the save line is empty, so the bar does not change height — and
          move the button out from under the thumb — as the state changes (§6). */}
      <div className="flex flex-col gap-2">
        <SaveStatus state={saveState} />
        {children}
      </div>
    </ActionBar>
  )
}

function GenerationSection({
  post,
  beforeStart,
  saveState,
}: {
  post: PostDraft
  beforeStart: () => Promise<void>
  saveState: SaveState
}) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const generateRef = useRef<GenerateButtonHandle>(null)
  const reviseRef = useRef<ReviseFormHandle>(null)
  const resultRef = useRef<HTMLDivElement>(null)
  const refreshedSnapshot = useRef('')
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || post.activeJob?.id || ''
  const postKey = useMemo(() => getPostQueryKey(transport, post.slug), [post.slug, transport])
  const postsKey = useMemo(() => listPostsQueryKey(transport), [transport])
  const invalidateOnDone = useMemo(() => [postKey, postsKey], [postKey, postsKey])
  const jobState = useJob(jobId, invalidateOnDone)
  const job = jobState.job ?? (post.activeJob?.id === jobId ? post.activeJob : undefined)
  const result = hasContent(post) ? post.content : undefined

  // Observations are persisted batch-by-batch on the post, not on the job. Refresh that
  // read model whenever observe progress changes so the contact sheet fills while the
  // durable job is still running.
  useEffect(() => {
    if (!job || isTerminal(job)) return
    const snapshot = `${job.id}:${job.updatedAt}:${job.progressDone}:${job.progressTotal}`
    if (refreshedSnapshot.current === snapshot) return
    refreshedSnapshot.current = snapshot
    void queryClient.invalidateQueries({ queryKey: postKey })
  }, [job, postKey, queryClient])

  // The finished draft mounts about a screen and a half below the button that asked for it, so
  // "done" used to be signalled only by the progress line quietly disappearing — a phone user
  // reasonably reads that as a failure. The dock says it finished and the page goes there (§4.3).
  const announcedJob = useRef('')
  const scrollWanted = useRef(false)
  useEffect(() => {
    if (!job || job.status !== 'done' || announcedJob.current === job.id) return
    announcedJob.current = job.id
    scrollWanted.current = true
  }, [job])

  // No dependency list: the draft arrives on a later render than the one that turned the job
  // terminal (`useJob` invalidates the post, which then refetches), so this waits for whichever
  // render brings it and disarms itself once it has delivered.
  useEffect(() => {
    if (!scrollWanted.current || !resultRef.current) return
    scrollWanted.current = false
    showResult()
  })

  function showResult() {
    const node = resultRef.current
    // jsdom has no layout engine and therefore no scrollIntoView; the editor's tests walk this
    // exact path.
    if (!node || typeof node.scrollIntoView !== 'function') return
    node.scrollIntoView({ block: 'start' })
  }

  return (
    <>
      <section aria-labelledby="generation-heading" className="mt-10">
        <h2 id="generation-heading" className="text-lg font-semibold tracking-tight">
          글 생성
        </h2>
        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <div>
            <StageModelSelect stage="observe" optional={post.images.length === 0} />
            {post.images.length === 0 && (
              <p className="text-content-tertiary mt-1 text-xs">
                사진이 없어 관찰 모델은 필요하지 않아요.
              </p>
            )}
          </div>
          <div>
            <p className="text-sm font-medium">작성 A/B 모델</p>
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
      {result && (
        // `scroll-mt-4` so the jump lands the draft's heading clear of the top edge rather than
        // flush against it.
        <div ref={resultRef} className="scroll-mt-4">
          <BlockList content={result} images={post.images} />
          <ReviseForm
            ref={reviseRef}
            postSlug={post.slug}
            activeJob={job}
            jobPending={Boolean(jobId) && !job}
            onStarted={setStartedJobId}
          />
          <ExportPanel content={result} images={post.images} createdAt={post.createdAt} />
        </div>
      )}

      <EditorDock saveState={saveState}>
        {/* The job's state rides in the dock with the button that started it: the CTA is docked
            now, so this is where the user is looking when they press 생성 (§4.3). */}
        {jobId &&
          (jobState.isError ? (
            <FailureNotice error="작업 상태를 확인하지 못했어요." onRetry={jobState.refetch} />
          ) : job?.status === 'failed' ? (
            <FailureNotice
              error={job.error}
              onRetry={
                job.kind === 'revise' && startedJobId !== job.id
                  ? undefined
                  : () =>
                      job.kind === 'revise'
                        ? reviseRef.current?.start()
                        : generateRef.current?.start()
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
                  onClick={showResult}
                  className="text-notice-success-fg shrink-0 underline"
                >
                  결과 보기
                </Button>
              )}
            </Notice>
          ) : null)}
        <GenerateButton
          ref={generateRef}
          post={post}
          activeJob={job}
          jobPending={Boolean(jobId) && !job}
          onStarted={setStartedJobId}
          beforeStart={beforeStart}
        />
      </EditorDock>
    </>
  )
}
