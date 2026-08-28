import { useEffect, useMemo, useRef, useState } from 'react'
import { Link, useNavigate } from '@tanstack/react-router'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { FailureNotice, ProgressLine, isTerminal, useJob } from '@/entities/generation-job'
import {
  BlockList,
  getPostQueryKey,
  hasContent,
  postStatusLabel,
  type PostDraft,
} from '@/entities/post'
import { GenerateButton, type GenerateButtonHandle } from '@/features/generate-post'
import { ReviseForm, type ReviseFormHandle } from '@/features/edit-with-ai'
import { SaveStatus, peekPendingDraft, useAutosave } from '@/features/save-draft'
import { StageModelSelect } from '@/features/select-model'
import { Badge, FieldLabel, Textarea, TextField } from '@/shared/ui'
import { ContactSheet } from '@/widgets/contact-sheet'
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
  const titleRef = useRef<HTMLInputElement>(null)
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
      <div className="flex items-center justify-between">
        <Link
          to="/posts"
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
        >
          ← 글 목록
        </Link>
        <span className="flex items-center gap-2">
          {post && <Badge>{postStatusLabel(post.status)}</Badge>}
          <SaveStatus state={autosave.state} />
        </span>
      </div>

      <FieldLabel htmlFor="post-title" className="sr-only">
        제목
      </FieldLabel>
      <TextField
        id="post-title"
        ref={titleRef}
        appearance="bare"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
        placeholder="제목"
        className="mt-4 min-h-11 text-2xl font-semibold tracking-tight"
      />

      <FieldLabel htmlFor="post-memo" className="sr-only">
        메모
      </FieldLabel>
      <Textarea
        id="post-memo"
        ref={memoRef}
        appearance="bare"
        value={memo}
        onChange={(event) => setMemo(event.target.value)}
        placeholder="무슨 일이 있었는지 편하게 적어 주세요"
        rows={16}
        className="mt-5 text-sm leading-relaxed"
      />

      <EditorPhotos post={post} ensureSlug={autosave.ensureSlug} />

      {post && <GenerationSection post={post} beforeStart={autosave.flush} />}
    </main>
  )
}

function GenerationSection({
  post,
  beforeStart,
}: {
  post: PostDraft
  beforeStart: () => Promise<void>
}) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const generateRef = useRef<GenerateButtonHandle>(null)
  const reviseRef = useRef<ReviseFormHandle>(null)
  const refreshedSnapshot = useRef('')
  const [startedJobId, setStartedJobId] = useState('')
  const jobId = startedJobId || post.activeJob?.id || ''
  const postKey = useMemo(() => getPostQueryKey(transport, post.slug), [post.slug, transport])
  const invalidateOnDone = useMemo(() => [postKey], [postKey])
  const jobState = useJob(jobId, invalidateOnDone)
  const job = jobState.job ?? (post.activeJob?.id === jobId ? post.activeJob : undefined)

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
          <StageModelSelect stage="write" />
        </div>
        <div className="mt-5">
          <GenerateButton
            ref={generateRef}
            post={post}
            activeJob={job}
            jobPending={Boolean(jobId) && !job}
            onStarted={setStartedJobId}
            beforeStart={beforeStart}
          />
        </div>
      </section>

      {jobId && (
        <section className="mt-6" aria-label="글 생성 상태">
          {jobState.isError ? (
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
          ) : null}
        </section>
      )}

      {post.images.length > 0 && (
        <ContactSheet images={post.images} observations={post.observations} activeJob={job} />
      )}
      {hasContent(post) && post.content && (
        <>
          <BlockList content={post.content} images={post.images} />
          <ReviseForm
            ref={reviseRef}
            postSlug={post.slug}
            activeJob={job}
            jobPending={Boolean(jobId) && !job}
            onStarted={setStartedJobId}
          />
        </>
      )}
    </>
  )
}
