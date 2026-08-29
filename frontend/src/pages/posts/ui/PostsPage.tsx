import { Link } from '@tanstack/react-router'
import { displayTitle, postStatusLabel, usePosts } from '@/entities/post'
import { useExperiments } from '@/entities/model-experiment'
import { formatRelativeTime } from '@/shared/lib'
import { Badge, Button, buttonStyles } from '@/shared/ui'

/** The way back to unfinished work (PRD F-8). The server returns only the acting user's
 *  posts, newest first — this screen does not sort or filter. */
export function PostsPage() {
  const { posts, isPending, isError, refetch } = usePosts()
  const { experiments } = useExperiments()
  const byId = new Map(experiments.map((experiment) => [experiment.id, experiment]))

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-6 sm:px-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">내 글</h1>
        <Link to="/posts/new" className={buttonStyles({ variant: 'cta' })}>
          새 글
        </Link>
      </div>

      {isError && (
        <div
          role="alert"
          className="bg-notice-danger-bg text-notice-danger-fg mt-8 flex flex-wrap items-center gap-2 rounded-md px-3 py-2 text-sm"
        >
          <span>목록을 불러오지 못했어요.</span>
          <Button variant="ghost" onClick={refetch} className="text-notice-danger-fg underline">
            다시 시도
          </Button>
        </div>
      )}

      {!isError && isPending && <p className="text-content-tertiary mt-8 text-sm">불러오는 중…</p>}

      {!isError && !isPending && posts.length === 0 && (
        <p className="text-content-tertiary mt-8 text-sm">
          아직 글이 없어요. "새 글"로 시작해 보세요.
        </p>
      )}

      <ul className="divide-divider mt-4 divide-y">
        {posts.map((post) => {
          const pending = post.pendingExperimentId ? byId.get(post.pendingExperimentId) : undefined
          const content = (
            <>
              <span className="min-w-0 flex-1 truncate text-sm">{displayTitle(post)}</span>
              <Badge>
                {post.activeJob
                  ? 'AI 생성 중'
                  : pending?.status === 'failed' || pending?.status === 'partial'
                    ? 'AI 결과 오류'
                    : post.pendingExperimentId
                      ? 'AI 결과 확인'
                      : postStatusLabel(post.status)}
              </Badge>
              <time dateTime={post.updatedAt} className="text-content-tertiary shrink-0 text-xs">
                {formatRelativeTime(post.updatedAt)}
              </time>
            </>
          )
          return (
            <li key={post.slug}>
              {post.pendingExperimentId && !post.activeJob ? (
                <Link
                  to="/ai-models/experiments/$id"
                  params={{ id: post.pendingExperimentId }}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center gap-3 px-2 py-3"
                >
                  {content}
                </Link>
              ) : (
                <Link
                  to="/posts/$slug"
                  params={{ slug: post.slug }}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center gap-3 px-2 py-3"
                >
                  {content}
                </Link>
              )}
            </li>
          )
        })}
      </ul>
    </main>
  )
}
