import type { ReactNode } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { useTransport } from '@connectrpc/connect-query'
import { FailureNotice, ProgressLine, useJob, type GenerationJob } from '@/entities/generation-job'
import { getPostQueryKey, type PostLoadFailure, usePost } from '@/entities/post'
import { Button } from '@/shared/ui'
import { DraftEditor } from './DraftEditor'

/** `/posts/new` — a draft that does not exist yet. The first autosave creates it and
 *  moves the URL to the minted slug (see DraftEditor's `onMinted`). */
export function NewDraftPage() {
  return <DraftEditor />
}

/** `/posts/$slug` — an existing post. */
export function PostEditorPage() {
  // The '/authenticated' prefix is the id of the pathless guard layout every signed-in
  // route hangs off (app/routes/router.tsx) — the URL itself is still '/posts/<slug>'.
  const { slug } = useParams({ from: '/authenticated/posts/$slug' })
  const { post, isPending, failure, refetch } = usePost(slug)

  if (failure) return <LoadFailure failure={failure} onRetry={refetch} />
  if (isPending) return <EditorPlaceholder>불러오는 중…</EditorPlaceholder>
  // A 200 carrying no post is not an answer the editor can work with; treating it as an
  // empty draft would autosave over whatever the post actually holds.
  if (!post) return <LoadFailure failure="not-found" onRetry={refetch} />

  // Keyed by slug: this route stays mounted when the param changes, so opening another
  // post from the list has to start a new editor rather than keep the previous text.
  return (
    <DraftEditor
      key={slug}
      post={post}
      status={post.activeJob ? <ActiveJobStatus initial={post.activeJob} slug={slug} /> : undefined}
    />
  )
}

function ActiveJobStatus({ initial, slug }: { initial: GenerationJob; slug: string }) {
  const transport = useTransport()
  const invalidationKeys = [getPostQueryKey(transport, slug)]
  const { job, isError, refetch } = useJob(initial.id, invalidationKeys)
  const current = job ?? initial

  if (isError) {
    return <FailureNotice error="작업 상태를 확인하지 못했어요." onRetry={refetch} />
  }
  if (current.status === 'failed') return <FailureNotice error={current.error} />
  return <ProgressLine job={current} />
}

const FAILURE_MESSAGES: Record<PostLoadFailure, string> = {
  forbidden: '다른 사람의 글이에요.',
  'not-found': '없는 글이에요.',
  unavailable: '글을 불러오지 못했어요.',
}

function LoadFailure({ failure, onRetry }: { failure: PostLoadFailure; onRetry: () => void }) {
  return (
    <EditorPlaceholder>
      <span role="alert" className="text-notice-danger-fg">
        {FAILURE_MESSAGES[failure]}
      </span>
      <span className="mt-4 flex items-center gap-2">
        <Link
          to="/posts"
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
        >
          글 목록으로
        </Link>
        {/* Only for a failure that is not an answer: retrying a 403 or a 404 would just
            ask the same question again. */}
        {failure === 'unavailable' && (
          <Button variant="ghost" onClick={onRetry} className="underline">
            다시 시도
          </Button>
        )}
      </span>
    </EditorPlaceholder>
  )
}

function EditorPlaceholder({ children }: { children: ReactNode }) {
  return (
    <main className="text-content-tertiary mx-auto flex w-full max-w-2xl flex-col items-start px-4 py-16 text-sm sm:px-6">
      {children}
    </main>
  )
}
