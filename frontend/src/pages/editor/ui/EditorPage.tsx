import type { ReactNode } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { type PostLoadFailure, usePost } from '@/entities/post'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { Button } from '@/shared/ui'
import { DraftEditor } from './DraftEditor'

/** `/posts/new` — a draft that does not exist yet. The first autosave creates it and
 *  moves the URL to the minted slug (see DraftEditor's `onMinted`).
 *
 *  The directory is read first: a create must name its voice, and the picker is initialized to
 *  the account's default (spec/policy/posts.md), so the editor waits for that one answer rather
 *  than letting a first keystroke send a save with no voice for the server to refuse. */
export function NewDraftPage() {
  const { user } = useSession()
  const { defaultVoice, isPending, isError, refetch } = useVoices(user?.id ?? '')

  if (isError) return <LoadFailure message="말투 목록을 불러오지 못했어요." onRetry={refetch} />
  if (isPending) return <EditorPlaceholder>불러오는 중…</EditorPlaceholder>
  if (!defaultVoice) {
    return (
      <LoadFailure
        message="기본 말투가 없어요. 말투 목록에서 하나를 기본으로 설정해 주세요."
        action={
          <Link
            to="/voices"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
          >
            말투 목록으로
          </Link>
        }
      />
    )
  }
  return <DraftEditor defaultVoiceId={defaultVoice.id} />
}

/** `/posts/$slug` — an existing post. */
export function PostEditorPage() {
  // The '/authenticated' prefix is the id of the pathless guard layout every signed-in
  // route hangs off (app/routes/router.tsx) — the URL itself is still '/posts/<slug>'.
  const { slug } = useParams({ from: '/authenticated/posts/$slug' })
  const { post, isPending, failure, refetch } = usePost(slug)

  if (failure) {
    return (
      <LoadFailure
        message={FAILURE_MESSAGES[failure]}
        // Only for a failure that is not an answer: retrying a 403 or a 404 would just
        // ask the same question again.
        onRetry={failure === 'unavailable' ? refetch : undefined}
      />
    )
  }
  if (isPending) return <EditorPlaceholder>불러오는 중…</EditorPlaceholder>
  // A 200 carrying no post is not an answer the editor can work with; treating it as an
  // empty draft would autosave over whatever the post actually holds.
  if (!post) return <LoadFailure message={FAILURE_MESSAGES['not-found']} />

  // Keyed by slug: this route stays mounted when the param changes, so opening another
  // post from the list has to start a new editor rather than keep the previous text.
  // The empty-profile warning is the editor's own now — it belongs below the memo, inside the
  // page's one `<main>`, rather than above it as a sibling (see DraftEditor).
  return <DraftEditor key={slug} post={post} />
}

const FAILURE_MESSAGES: Record<PostLoadFailure, string> = {
  forbidden: '다른 사람의 글이에요.',
  'not-found': '없는 글이에요.',
  unavailable: '글을 불러오지 못했어요.',
}

function LoadFailure({
  message,
  onRetry,
  action,
}: {
  message: string
  onRetry?: () => void
  action?: ReactNode
}) {
  return (
    <EditorPlaceholder>
      <span role="alert" className="text-notice-danger-fg">
        {message}
      </span>
      <span className="mt-4 flex flex-wrap items-center gap-2">
        <Link
          to="/posts"
          className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
        >
          글 목록으로
        </Link>
        {action}
        {onRetry && (
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
