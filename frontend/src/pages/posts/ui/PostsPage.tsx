import { Link } from '@tanstack/react-router'
import { displayTitle, postStatusLabel, usePosts } from '@/entities/post'
import { formatRelativeTime } from '@/shared/lib'

/** The way back to unfinished work (PRD F-8). The server returns only the acting user's
 *  posts, newest first — this screen does not sort or filter. */
export function PostsPage() {
  const { posts, isPending, isError, refetch } = usePosts()

  return (
    <main className="mx-auto w-full max-w-2xl px-6 py-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold tracking-tight">내 글</h1>
        <Link
          to="/posts/new"
          className="rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary-hover"
        >
          새 글
        </Link>
      </div>

      {isError && (
        <p role="alert" className="mt-8 text-sm text-danger">
          목록을 불러오지 못했어요.{' '}
          <button type="button" onClick={refetch} className="underline">
            다시 시도
          </button>
        </p>
      )}

      {!isError && isPending && <p className="mt-8 text-sm text-text-subtle">불러오는 중…</p>}

      {!isError && !isPending && posts.length === 0 && (
        <p className="mt-8 text-sm text-text-subtle">아직 글이 없어요. "새 글"로 시작해 보세요.</p>
      )}

      <ul className="mt-4 divide-y divide-border">
        {posts.map((post) => (
          <li key={post.slug}>
            <Link
              to="/posts/$slug"
              params={{ slug: post.slug }}
              className="flex items-center gap-3 py-3 hover:bg-surface-hover/60"
            >
              <span className="min-w-0 flex-1 truncate text-sm">{displayTitle(post)}</span>
              <span className="rounded-sm bg-surface-raised px-1.5 py-0.5 text-[10px] text-text-muted">
                {postStatusLabel(post.status)}
              </span>
              <time dateTime={post.updatedAt} className="shrink-0 text-xs text-text-subtle">
                {formatRelativeTime(post.updatedAt)}
              </time>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  )
}
