import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import {
  displayTitle,
  isPostStatus,
  postStatusLabel,
  rememberPostEntry,
  usePostList,
  type PostListItem,
  type PostStatus,
} from '@/entities/post'
import { useExperiments, type ModelExperiment } from '@/entities/model-experiment'
import { isTerminal, progressLabel } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { matchedTags, PostListControls, type PostNarrowing } from '@/features/filter-posts'
import { TemplateRefLabel } from '@/entities/template'
import { VoiceRefLabel } from '@/entities/voice'
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

/** The one status chip a row carries. Colour never travels alone (THEME-18): the tone
 *  only reinforces the label, so the label is chosen first and the tone follows it. */
function rowStatus(
  post: PostListItem,
  pending: ModelExperiment | undefined,
  t: TFunction<'posts'>,
): { label: string; tone: BadgeTone } {
  if (post.activeJob && !isTerminal(post.activeJob))
    return { label: t('list.state.generating'), tone: 'info' }
  if (post.latestOrdinaryFailure || post.activeJob?.status === 'failed')
    return { label: t('list.state.failed'), tone: 'danger' }
  if (pending?.status === 'failed' || pending?.status === 'partial')
    return { label: t('list.state.failed'), tone: 'danger' }
  if (post.pendingExperimentId) return { label: t('list.state.review'), tone: 'warning' }
  return { label: postStatusLabel(post.status), tone: postStatusTone(post.status) }
}

/** 초안 · 검토 · 확정 sat in one grey and were indistinguishable at a glance. Draft stays neutral
 *  (nothing has happened yet), review takes the accent (the user is mid-way), finalized takes
 *  the tinted success (done writing). Published in the same tint read as 확정 at a glance, so the
 *  post that is finished for good takes the solid `done` plane; the label still carries the
 *  meaning on its own (THEME-18, THEME-29). */
const STATUS_TONE: Record<PostStatus, BadgeTone> = {
  draft: 'neutral',
  review: 'accent',
  finalized: 'success',
  published: 'done',
}

function postStatusTone(status: string): BadgeTone {
  return isPostStatus(status) ? STATUS_TONE[status] : 'neutral'
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
      path: '/posts',
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
        // Below the desk the group row is what names this place, and it is chrome stuck to the
        // top of the viewport — the page's own top padding under it is a second gap between a
        // title and the first control, which put the search a thumb-swipe below 내 글 (THEME-38).
        // The desk, where the row is a rail and the heading is the page's own, keeps `py-8`.
        className: 'flex flex-1 flex-col pt-4 sm:pt-6 lg:pt-8',
      })}
    >
      <div className="px-4 sm:px-6 lg:px-8">
        <Typography variant="display">{t('list.mine', { ns: 'posts' })}</Typography>
      </div>

      {/* On the screen at every post count (POST-68): a search that appears at some number of
          posts is a second layout for the same page, and the count it would appear at is exactly
          where someone starts needing it. */}
      <div className="px-4 sm:px-6 lg:mt-6 lg:px-8">
        <PostListControls narrowing={narrowing} onChange={narrow} />
      </div>

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

      {/* One live region for both states, so finishing the load is a text change inside it rather
          than two nodes swapping — a swap announces nothing to VoiceOver or TalkBack (THEME-33). */}
      {!isError && (isPending || (isEmpty && !isNarrowed)) && (
        <Typography
          variant="body"
          role="status"
          className="text-content-tertiary mt-8 px-4 sm:px-6 lg:px-8"
        >
          {isPending ? t('state.loading', { ns: 'common' }) : t('list.empty', { ns: 'posts' })}
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

      <ul
        aria-label={t('list.history.directory', { ns: 'posts' })}
        className="divide-divider mt-4 shrink-0 divide-y"
      >
        {narrowed.map(({ post, matchedTags }) => {
          const status = rowStatus(
            post,
            post.pendingExperimentId ? byId.get(post.pendingExperimentId) : undefined,
            t,
          )
          // On a phone: two lines, not three items competing on one. At 360px a single row left
          // the title ~146px — about ten Hangul — and because the badge label swings from 초안 to
          // AI 결과 확인 the cut point moved row to row, so the list read as a ragged column of
          // half-titles. The voice sits between the status and the time as metadata: which voice a post is in
          // is the one thing this list newly has to say, and a tombstone must say so on the row
          // itself (POST-25) — the name gives way before the badge or the time do. A post with
          // 말투 없음 shows none: an absence is not metadata.
          const content = (
            <>
              <Typography
                variant="label"
                className="text-content-primary w-full truncate lg:w-auto lg:min-w-0 lg:flex-1"
              >
                {displayTitle(post)}
              </Typography>
              <span className="flex w-full min-w-0 flex-wrap items-center gap-2 lg:w-auto lg:shrink-0 lg:justify-end">
                <Badge tone={status.tone}>{status.label}</Badge>
                {post.exportReady && (
                  <span className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}>
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
                  <span className={typographyStyles({ variant: 'meta', className: 'truncate' })}>
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
            </>
          )
          // Two stacked lines on a phone, one line on the desk. At 360px a single row left the
          // title ~146px, so the metadata gets its own line there; from `lg:` up the column is
          // ~1,000px wide and the same two lines read as a ragged double-height list with the
          // right two thirds of every row empty — which is the whole complaint about a phone
          // layout centred on a desk. The title takes the free space and the metadata settles
          // against the right edge, so status, voice and time line up down the list.
          const rowClass =
            'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 min-w-0 flex-1 flex-col items-start justify-center gap-1 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:gap-4 lg:px-8'
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
              className={
                runningJob || failure
                  ? 'flex flex-col'
                  : 'flex flex-col lg:flex-row lg:items-center'
              }
            >
              {post.pendingExperimentId && !runningJob && !failedJob ? (
                <Link
                  to="/tests/records/$id"
                  params={{ id: post.pendingExperimentId }}
                  search={{ entry: historyEntry }}
                  className={rowClass}
                  onClick={() => remember(post)}
                >
                  {content}
                </Link>
              ) : (
                <Link
                  to="/posts/$slug"
                  params={{ slug: post.slug }}
                  className={rowClass}
                  onClick={() => remember(post)}
                >
                  {content}
                </Link>
              )}
              {/* Each recovery target is a sibling of the work link. Export has no link of its
                  own: it lives in the opened post's 글 완성 step, one step press from wherever the
                  post opens (POST-64). */}
              <div className="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1 px-4 pb-2 sm:px-6 lg:px-8 lg:py-2">
                {(runningJob || failure) && (
                  <Typography
                    variant={runningJob ? 'meta' : 'body'}
                    className="text-content-secondary w-full"
                  >
                    {runningJob ? progressLabel(runningJob) : failure && formatAppFailure(failure)}
                  </Typography>
                )}
                <Link
                  to="/posts/$slug"
                  params={{ slug: post.slug }}
                  onClick={() => remember(post)}
                  className={buttonStyles({ variant: 'ghost', className: '-ml-3' })}
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
                    className={buttonStyles({ variant: 'ghost' })}
                  >
                    {t('list.history.result', { ns: 'posts' })}
                  </Link>
                )}
                {post.publishedUrl && (
                  <a
                    href={post.publishedUrl}
                    target="_blank"
                    rel="noreferrer"
                    className={buttonStyles({ variant: 'ghost' })}
                  >
                    {t('list.history.published', { ns: 'posts' })}
                  </a>
                )}
              </div>
            </li>
          )
        })}
      </ul>

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
    </main>
  )
}
