import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { clsx } from 'clsx'
import { CircleCheck, PenLine, Scale, Send, Sparkles, TriangleAlert } from 'lucide-react'
import {
  displayTitle,
  postRowState,
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
  StatusTile,
  WorkGroup,
  workCardActionClass,
  workCardLinkClass,
  workCardStyles,
  workCardFooterClass,
} from '@/widgets/work-list'
import { POSTS_SEARCH_DEBOUNCE_MS } from '@/shared/config'
import { formatAppFailure, formatRelativeTime } from '@/shared/lib'
import {
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  typographyStyles,
  type BadgeTone,
  pageStyles,
  useNearViewport,
} from '@/shared/ui'
import { useSettledValue } from '../model/useSettledValue'
import { useHistoryScrollReturn } from '../model/useHistoryScrollReturn'

/** The post keeps its state-specific glyph; the tile's geometry and semantic tones are shared. */
function postStatusIcon(tone: BadgeTone, status: string) {
  return tone === 'info'
    ? Sparkles
    : tone === 'danger'
      ? TriangleAlert
      : tone === 'warning'
        ? Scale
        : status === 'published'
          ? Send
          : status === 'finalized'
            ? CircleCheck
            : status === 'review'
              ? Sparkles
              : PenLine
}

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
  const historyEntry = `/naver-blog${historyFilters.size ? '?' + historyFilters : ''}`
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
      to: '/naver-blog',
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
      path: '/naver-blog',
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
      {/* The channel opens straight onto this list: the marked channel link names the place, so
          the heading is for assistive technology only (THEME-38, POST-43). */}
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
        <Typography
          variant="body"
          role="status"
          className="text-content-tertiary mt-8 px-4 sm:px-6 lg:px-8"
        >
          {t('list.empty', { ns: 'posts' })}
        </Typography>
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
          // One post's card: the status tile, the title in a heading's size, its state as a chip,
          // and the recovery links under it. Continuation stays a compact link on every card.
          const renderPost = (
            { post, matchedTags }: (typeof narrowed)[number],
            latestCard = false,
          ) => {
            // The one chip a row carries, chosen the way aggregate 내 글 chooses it (POST-127).
            const status = postRowState(
              post,
              post.pendingExperimentId ? byId.get(post.pendingExperimentId)?.status : undefined,
            )
            // The title gets a whole line in a heading's size, and the metadata wraps under it: the
            // voice sits between the status and the time, because which voice a post is in is the
            // one thing this list has to say and a tombstone must say so on the card itself
            // (POST-25). A post with 말투 없음 shows none: an absence is not metadata.
            const content = (
              <>
                <StatusTile tone={status.tone} icon={postStatusIcon(status.tone, post.status)} />
                <span className="flex min-w-0 flex-1 flex-col gap-2">
                  <Typography
                    variant="fieldTitle"
                    as="span"
                    className="text-content-primary line-clamp-2"
                  >
                    {displayTitle(post)}
                  </Typography>
                  <span className="flex w-full min-w-0 flex-wrap items-center gap-2">
                    <Badge tone={status.tone}>{status.label}</Badge>
                    {post.exportReady && (
                      <span
                        className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}
                      >
                        {t('list.history.exportReady', { ns: 'posts' })}
                      </span>
                    )}
                    {post.voice && (
                      <VoiceRefLabel
                        voice={post.voice}
                        className={typographyStyles({ variant: 'meta' })}
                      />
                    )}
                    {/* Only for an assigned post, and after the voice: most rows carry a voice and
                    fewer a 템플릿, so it reads as an addition rather than a second column. */}
                    <TemplateRefLabel
                      template={post.template}
                      className={typographyStyles({ variant: 'meta' })}
                    />
                    {/* Only the tags the search actually matched, and only while it did (POST-65).
                    A row kept by a word its title never shows looks arbitrary otherwise; as
                    metadata rather than the chips ② uses, because a fourth object in a 360px
                    row is what pushed the title down to ten Hangul in the first place. */}
                    {matchedTags.length > 0 && (
                      <span
                        className={typographyStyles({ variant: 'meta', className: 'truncate' })}
                      >
                        {matchedTags.map((tag) => `#${tag}`).join(' ')}
                      </span>
                    )}
                    <time
                      dateTime={post.updatedAt}
                      className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}
                    >
                      {formatRelativeTime(post.updatedAt)}
                    </time>
                  </span>
                </span>
              </>
            )
            const runningJob =
              post.activeJob && !isTerminal(post.activeJob) ? post.activeJob : undefined
            const failedJob =
              post.latestOrdinaryFailure ??
              (post.activeJob?.status === 'failed' ? post.activeJob : undefined)
            const failure = failedJob?.failure
            return (
              <li
                key={post.slug}
                data-post-slug={post.slug}
                className={workCardStyles({ latest: latestCard })}
              >
                {post.pendingExperimentId && !runningJob && !failedJob ? (
                  <Link
                    to="/tests/records/$id"
                    params={{ id: post.pendingExperimentId }}
                    search={{ entry: historyEntry }}
                    className={workCardLinkClass}
                    onClick={() => remember(post)}
                  >
                    {content}
                  </Link>
                ) : (
                  <Link
                    to="/posts/$slug"
                    params={{ slug: post.slug }}
                    className={workCardLinkClass}
                    onClick={() => remember(post)}
                  >
                    {content}
                  </Link>
                )}
                {/* Each recovery target is a sibling of the work link. Export has no link of its
                  own: it lives in the opened post's 글 완성 step, one step press from wherever the
                  post opens (POST-64). */}
                <div className={workCardFooterClass}>
                  {(runningJob || failure) && (
                    <Typography
                      variant={runningJob ? 'meta' : 'body'}
                      className="text-content-secondary w-full"
                    >
                      {runningJob
                        ? progressLabel(runningJob)
                        : failure && formatAppFailure(failure)}
                    </Typography>
                  )}
                  <Link
                    to="/posts/$slug"
                    params={{ slug: post.slug }}
                    onClick={() => remember(post)}
                    className={buttonStyles({
                      variant: 'ghost',
                      className: clsx(
                        '-ml-3',
                        workCardActionClass,
                        latestCard && 'text-link-fg-current',
                      ),
                    })}
                  >
                    {t(
                      post.status === 'published' || post.status === 'finalized' || runningJob
                        ? 'list.history.open'
                        : 'list.history.continue',
                      { ns: 'posts' },
                    )}
                  </Link>
                  {post.pendingExperimentId && (runningJob || failedJob) && (
                    <Link
                      to="/tests/records/$id"
                      params={{ id: post.pendingExperimentId }}
                      search={{ entry: historyEntry }}
                      onClick={() => remember(post)}
                      className={buttonStyles({ variant: 'ghost', className: workCardActionClass })}
                    >
                      {t('list.history.result', { ns: 'posts' })}
                    </Link>
                  )}
                  {post.publishedUrl && (
                    <a
                      href={post.publishedUrl}
                      target="_blank"
                      rel="noreferrer"
                      className={buttonStyles({ variant: 'ghost', className: workCardActionClass })}
                    >
                      {t('list.history.published', { ns: 'posts' })}
                    </a>
                  )}
                </div>
              </li>
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
          <Typography variant="meta" role="status" className="text-content-tertiary">
            {list.isFetchingNextPage
              ? t('list.loadingMore', { ns: 'posts' })
              : list.isFetchNextPageError
                ? t('list.loadMoreFailed', { ns: 'posts' })
                : ''}
          </Typography>
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
