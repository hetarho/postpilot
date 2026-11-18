import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { ExternalLink } from 'lucide-react'
import {
  displayTitle,
  postRowState,
  POST_LIST_PATH,
  rememberPostEntry,
  usePostList,
  type PostListItem,
} from '@/entities/post'
import { useExperiments } from '@/entities/model-experiment'
import { isTerminal, progressLabel } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { matchedTags, PostListControls, type PostNarrowing } from '@/features/filter-posts'
import { TemplateRefLabel } from '@/entities/template'
import { VoiceRefLabel } from '@/entities/voice'
import { ChannelActions } from '@/widgets/channel-actions'
import {
  WorkListBody,
  WorkListToolbar,
  WorkListLoading,
  FindSheet,
  WorkGroup,
} from '@/widgets/work-list'
import { POSTS_SEARCH_DEBOUNCE_MS } from '@/shared/config'
import { formatAppFailure, formatRelativeTime } from '@/shared/lib'
import { postsIllustration } from '@/shared/assets'
import {
  Badge,
  Button,
  ListEmptyState,
  Notice,
  Spinner,
  Typography,
  WorkRow,
  buttonStyles,
  pageStyles,
  useNearViewport,
} from '@/shared/ui'
import { useSettledValue } from '../model/useSettledValue'
import { useHistoryScrollReturn } from '../model/useHistoryScrollReturn'

/** The way back to unfinished work (PRD F-8). The server returns only the acting user's posts,
 *  newest first, a page at a time as the list's end nears the screen (POST-90), and it applies the
 *  search and the status filter over every post the account owns (POST-91). The narrowing reads
 *  and writes the URL so it survives opening a post, coming back, a reload and a shared link
 *  (POST-67). */
export function PostsPage() {
  const { t } = useTranslation(['posts', 'common'])
  const { user } = useSession()
  const narrowing: PostNarrowing = useSearch({ strict: false })
  // The URL follows every keystroke; the request waits for the typing to stop. The status is one
  // choice, not typing, so it is sent at once.
  const historyFilters = new URLSearchParams()
  if (narrowing.q) historyFilters.set('q', narrowing.q)
  if (narrowing.status) historyFilters.set('status', narrowing.status)
  const historyEntry = `/posts${historyFilters.size ? '?' + historyFilters : ''}`
  const settledQ = useSettledValue(narrowing.q ?? '', POSTS_SEARCH_DEBOUNCE_MS)
  const list = usePostList({ q: settledQ, status: narrowing.status })
  useHistoryScrollReturn(user?.id ?? '', narrowing, settledQ, list)
  const { posts, isPending, isFetching, refetch } = list
  const { experiments } = useExperiments()
  const byId = new Map(experiments.map((experiment) => [experiment.id, experiment]))
  const navigate = useNavigate()
  // `replace`, not a push: a history entry per keystroke would make 뒤로 mean "one character
  // ago" instead of "the screen I came from". An emptied field drops the param rather than
  // carrying `?q=`.
  const narrow = (next: PostNarrowing) =>
    void navigate({
      to: '/posts',
      search: { q: next.q?.trim() === '' ? undefined : next.q, status: next.status },
      replace: true,
    })
  const narrowed = posts.map((post) => ({ post, matchedTags: matchedTags(post, settledQ) }))
  // Guide: 완성 is a post finished writing (확정 or 발행) with no run going; everything else is
  // still 작성 중. The latest unfinished post is emphasized inside that group unless a narrowing is on.
  const finished = ({ post }: (typeof narrowed)[number]) =>
    (post.status === 'finalized' || post.status === 'published') &&
    !(post.activeJob && !isTerminal(post.activeJob))
  const isNarrowedNow = (narrowing.q ?? '').trim() !== '' || narrowing.status !== undefined
  const latest = isNarrowedNow ? undefined : narrowed.find((item) => !finished(item))
  const groups = [
    { key: 'inProgress', items: narrowed.filter((item) => !finished(item)) },
    { key: 'done', items: narrowed.filter(finished) },
  ] as const
  // The page-top notice is for a list with nothing to show; a page further down that fails keeps
  // every row on screen and says so at the list's end instead (POST-92).
  const isError = list.isError && posts.length === 0
  const isNarrowed = settledQ.trim() !== '' || narrowing.status !== undefined
  const isEmpty = !isPending && !isError && posts.length === 0
  // An account with nothing to show under a narrowing is told about the narrowing, which offers the
  // way back; the account's own emptiness is said once the narrowing is gone (POST-69).
  const noMatch = isEmpty && isNarrowed
  // The next page loads while the list's end is within a screen, one page at a time, and a failed
  // page waits for 다시 시도 rather than retrying on every scroll.
  const listEnd = useNearViewport<HTMLDivElement>(
    list.fetchNextPage,
    list.hasNextPage && !list.isFetchingNextPage && !list.isFetchNextPageError,
  )
  const statusLabel = narrowing.status ? t(`list.filter.${narrowing.status}`, { ns: 'posts' }) : ''
  const remember = (post: PostListItem) => {
    if (!user?.id) return
    rememberPostEntry(user.id, {
      path: POST_LIST_PATH,
      section: 'posts',
      filters: {
        ...(narrowing.q ? { q: narrowing.q } : {}),
        ...(narrowing.status ? { status: narrowing.status } : {}),
      },
      scrollY: window.scrollY,
      targetId: post.slug,
    })
  }
  const noMatchText = narrowing.q?.trim()
    ? narrowing.status
      ? t('list.noMatch.both', { ns: 'posts', q: narrowing.q.trim(), status: statusLabel })
      : t('list.noMatch.query', { ns: 'posts', q: narrowing.q.trim() })
    : t('list.noMatch.status', { ns: 'posts', status: statusLabel })

  return (
    // The page gutter lives on each block rather than on `main`, so the list rows can run edge to
    // edge: a pressed row that stops 16px short of the screen edge reads as a card, and a row inset
    // deeper than the page's own rhythm reads as a mistake (THEME-23).
    <main
      className={pageStyles({
        width: 'wide',
        gutters: false,
        className: 'flex flex-1 flex-col pt-2 sm:pt-4',
      })}
    >
      {/* The shell draws the 글 tab's large title over this list, so the page's own h1 names the
          same tab for the outline only (THEME-38, POST-43). */}
      <h1 className="sr-only">{t('list.mine', { ns: 'posts' })}</h1>

      {/* Guide: the search and the status filter wait behind ONE 찾기 button (a bottom sheet), so
          the list opens on its work. The button is there at every post count, and says how many
          narrowings are on while they are. */}
      <WorkListToolbar className="px-4 sm:px-6 lg:px-8">
        <FindSheet
          active={(narrowing.q?.trim() ? 1 : 0) + (narrowing.status ? 1 : 0)}
          onClear={() => narrow({})}
        >
          <PostListControls narrowing={narrowing} onChange={narrow} />
        </FindSheet>
      </WorkListToolbar>

      {isError && (
        <Notice tone="danger" role="alert" className="mx-4 mt-8 sm:mx-6">
          <span>{t('list.loadFailed', { ns: 'posts' })}</span>
          {/* `isFetching`, not `isPending`: react-query keeps `status: 'error'` across a refetch of
              an errored query, so without it the notice does not move a pixel for the several
              seconds a retry takes on cellular and the user taps it again and again (THEME-28). */}
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}

      {/* Empty-account feedback; the shared list body owns loading geometry and its live region. */}
      {!isError && isEmpty && !isNarrowed && (
        <ListEmptyState illustration={postsIllustration} className="mt-6">
          {t('list.empty', { ns: 'posts' })}
        </ListEmptyState>
      )}

      {/* A narrowing that matches nothing is NOT the same screen as an account with no posts
          (POST-69): it names what is narrowing, so the user can see it is their own query and
          not an empty account, and it offers the one way back to the whole list. Page text, no
          card and no illustration. */}
      {noMatch && (
        <div className="mt-8 px-4 sm:px-6 lg:px-8">
          <Typography variant="body" role="status" className="text-content-tertiary">
            {noMatchText}
          </Typography>
          <Button variant="ghost" onClick={() => narrow({})} className="mt-2 -ml-3">
            {t('list.reset', { ns: 'posts' })}
          </Button>
        </div>
      )}

      {/* Every item appears once; latest-item emphasis never adds another section or density. */}
      <WorkListBody
        label={t('list.history.directory', { ns: 'posts' })}
        className="px-4 sm:px-6 lg:px-8"
      >
        <WorkListLoading pending={isPending} />
        {(() => {
          // One post's work row (POST-64): the whole row opens the post — or the pending
          // comparison's record while it waits — and its one trailing action opens somewhere else.
          const renderPost = (
            { post, matchedTags }: (typeof narrowed)[number],
            latestRow = false,
          ) => {
            // The one chip a row carries, chosen the way aggregate 내 글 chooses it (POST-127).
            const status = postRowState(
              post,
              post.pendingExperimentId ? byId.get(post.pendingExperimentId)?.status : undefined,
            )
            const runningJob =
              post.activeJob && !isTerminal(post.activeJob) ? post.activeJob : undefined
            const failedJob =
              post.latestOrdinaryFailure ??
              (post.activeJob?.status === 'failed' ? post.activeJob : undefined)
            const failure = failedJob?.failure
            // A comparison waiting on a settled post is where its row goes, as it always has.
            const recordId = !runningJob && !failedJob ? post.pendingExperimentId : undefined
            const meta = (
              <>
                <Badge tone={status.tone} className="shrink-0">
                  {status.label}
                </Badge>
                {/* Exportable is the post's state, said beside its badge whatever else runs. */}
                {post.exportReady && (
                  <span className="shrink-0">{t('list.history.exportReady', { ns: 'posts' })}</span>
                )}
                {/* A running job's progress or a failure takes the made-with and time's place. */}
                {runningJob || failure ? (
                  <span className="min-w-0 break-words">
                    {runningJob ? progressLabel(runningJob) : failure && formatAppFailure(failure)}
                  </span>
                ) : (
                  <>
                    {/* Which voice a post is in is what this list has to say, and a tombstone says
                        so on the row itself (POST-25); 말투 없음 shows none. */}
                    {post.voice && (
                      <VoiceRefLabel voice={post.voice} className="max-w-full min-w-0 truncate" />
                    )}
                    <TemplateRefLabel
                      template={post.template}
                      className="max-w-full min-w-0 truncate"
                    />
                    {/* Only the tags the search matched, and only while it did (POST-65). */}
                    {matchedTags.length > 0 && (
                      <span className="max-w-full min-w-0 truncate">
                        {matchedTags.map((tag) => `#${tag}`).join(' ')}
                      </span>
                    )}
                    <time dateTime={post.updatedAt} className="shrink-0">
                      {formatRelativeTime(post.updatedAt)}
                    </time>
                  </>
                )}
              </>
            )
            // At most one action, never a second link to the row's own address: the comparison
            // result while one waits behind a running or failed row, else the published post.
            // Export has no link of its own: it lives in the opened post's 글 완성 step (POST-64).
            const action =
              post.pendingExperimentId && !recordId ? (
                <Link
                  to="/tests/records/$id"
                  params={{ id: post.pendingExperimentId }}
                  search={{ entry: historyEntry }}
                  onClick={() => remember(post)}
                  className={buttonStyles({ variant: 'ghost', className: 'px-3' })}
                >
                  {t('list.history.result', { ns: 'posts' })}
                </Link>
              ) : post.publishedUrl ? (
                <a
                  href={post.publishedUrl}
                  target="_blank"
                  rel="noreferrer"
                  className={buttonStyles({ variant: 'ghost', className: 'gap-1 px-3' })}
                >
                  {t('list.history.published', { ns: 'posts' })}
                  <ExternalLink aria-hidden="true" className="size-4" />
                </a>
              ) : undefined
            return (
              <WorkRow
                key={post.slug}
                data-post-slug={post.slug}
                tint={latestRow}
                cue={latestRow ? t('work.resume', { ns: 'channel' }) : undefined}
                title={displayTitle(post)}
                meta={meta}
                action={action}
                link={({ className, children }) =>
                  recordId ? (
                    <Link
                      to="/tests/records/$id"
                      params={{ id: recordId }}
                      search={{ entry: historyEntry }}
                      className={className}
                      onClick={() => remember(post)}
                    >
                      {children}
                    </Link>
                  ) : (
                    <Link
                      to="/posts/$slug"
                      params={{ slug: post.slug }}
                      className={className}
                      onClick={() => remember(post)}
                    >
                      {children}
                    </Link>
                  )
                }
              />
            )
          }
          return (
            <>
              {groups.map(
                (group) =>
                  group.items.length > 0 && (
                    <WorkGroup
                      key={group.key}
                      heading={t(`work.${group.key}`, { ns: 'channel' })}
                      count={group.items.length}
                    >
                      {group.items.map((item) => renderPost(item, item === latest))}
                    </WorkGroup>
                  ),
              )}
            </>
          )
        })()}
      </WorkListBody>

      {/* The list's end, while there is more to load: what the observer watches, and ONE live region
          whose text says a page is loading or that it failed, so the change is announced rather than
          a node swapped in (THEME-33). The retry asks for that page again and nothing else (POST-92). */}
      {posts.length > 0 && list.hasNextPage && (
        <div ref={listEnd} className="px-4 py-4 sm:px-6 lg:px-8">
          <div className="text-content-tertiary flex items-center gap-2">
            {list.isFetchingNextPage && (
              <span aria-busy="true" className="loading-reveal flex">
                <Spinner />
              </span>
            )}
            <Typography variant="meta" role="status">
              {list.isFetchingNextPage ? (
                <span className="loading-reveal">{t('list.loadingMore', { ns: 'posts' })}</span>
              ) : list.isFetchNextPageError ? (
                t('list.loadMoreFailed', { ns: 'posts' })
              ) : (
                ''
              )}
            </Typography>
          </div>
          {list.isFetchNextPageError && (
            <Button
              variant="ghost"
              onClick={list.fetchNextPage}
              pending={list.isFetchingNextPage}
              className="mt-1 -ml-3"
            >
              {t('action.retry', { ns: 'common' })}
            </Button>
          )}
        </div>
      )}
      <ChannelActions section="work" className="action-list-inset mt-auto" />
    </main>
  )
}
