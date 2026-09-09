import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { displayTitle, postStatusLabel, usePosts, type PostListItem } from '@/entities/post'
import { useExperiments, type ModelExperiment } from '@/entities/model-experiment'
import { narrowPosts, PostListControls, type PostNarrowing } from '@/features/filter-posts'
import { TemplateRefLabel } from '@/entities/template'
import { VoiceRefLabel } from '@/entities/voice'
import { formatRelativeTime } from '@/shared/lib'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  typographyStyles,
  type BadgeTone,
  pageStyles,
} from '@/shared/ui'

/** The one status chip a row carries. Colour never travels alone (design-language §2.6): the tone
 *  only reinforces the label, so the label is chosen first and the tone follows it. */
function rowStatus(
  post: PostListItem,
  pending: ModelExperiment | undefined,
  t: TFunction<'posts'>,
): { label: string; tone: BadgeTone } {
  if (post.activeJob) return { label: t('list.state.generating'), tone: 'info' }
  if (pending?.status === 'failed' || pending?.status === 'partial')
    return { label: t('list.state.failed'), tone: 'danger' }
  if (post.pendingExperimentId) return { label: t('list.state.review'), tone: 'warning' }
  return { label: postStatusLabel(post.status), tone: postStatusTone(post.status) }
}

/** 초안 · 검토 · 확정 sat in one grey and were indistinguishable at a glance. Draft stays neutral
 *  (nothing has happened yet), review takes the accent (the user is mid-way), finalized takes
 *  success (done). The label still carries the meaning on its own (§2.6). */
function postStatusTone(status: string): BadgeTone {
  if (status === 'review') return 'accent'
  if (status === 'finalized') return 'success'
  return 'neutral'
}

/** The way back to unfinished work (PRD F-8). The server returns only the acting user's posts,
 *  newest first and all of them; the search and the status filter narrow that one answer here in
 *  the browser (POST-65, POST-66), reading and writing the URL so the narrowing survives opening
 *  a post, coming back, a reload and a shared link (POST-67). */
export function PostsPage() {
  const { t } = useTranslation(['posts', 'common'])
  const { posts, isPending, isFetching, isError, refetch } = usePosts()
  const { experiments } = useExperiments()
  const byId = new Map(experiments.map((experiment) => [experiment.id, experiment]))
  const narrowing: PostNarrowing = useSearch({ from: '/authenticated/posts' })
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
  const narrowed = narrowPosts(posts, narrowing)
  const noMatch = !isPending && !isError && posts.length > 0 && narrowed.length === 0
  const statusLabel = narrowing.status ? t(`list.filter.${narrowing.status}`, { ns: 'posts' }) : ''
  const noMatchText = narrowing.q?.trim()
    ? narrowing.status
      ? t('list.noMatch.both', { ns: 'posts', q: narrowing.q.trim(), status: statusLabel })
      : t('list.noMatch.query', { ns: 'posts', q: narrowing.q.trim() })
    : t('list.noMatch.status', { ns: 'posts', status: statusLabel })

  return (
    // The page gutter lives on each block rather than on `main`, so the list rows can run edge to
    // edge: a pressed row that stops 16px short of the screen edge reads as a card, and a row inset
    // deeper than the page's own rhythm reads as a mistake (design-language §4.2).
    <main
      className={pageStyles({ width: 'wide', gutters: false, className: 'flex flex-1 flex-col' })}
    >
      <div className="px-4 sm:px-6 lg:px-8">
        <Typography variant="display">{t('list.mine', { ns: 'posts' })}</Typography>
      </div>

      {/* On the screen at every post count (POST-68): a search that appears at some number of
          posts is a second layout for the same page, and the count it would appear at is exactly
          where someone starts needing it. */}
      <div className="mt-6 px-4 sm:px-6 lg:px-8">
        <PostListControls narrowing={narrowing} onChange={narrow} />
      </div>

      {isError && (
        <Notice tone="danger" role="alert" className="mx-4 mt-8 sm:mx-6">
          <span>{t('list.loadFailed', { ns: 'posts' })}</span>
          {/* `isFetching`, not `isPending`: react-query keeps `status: 'error'` across a refetch of
              an errored query, so without it the notice does not move a pixel for the several
              seconds a retry takes on cellular and the user taps it again and again (§6). */}
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
          than two nodes swapping — a swap announces nothing to VoiceOver or TalkBack (§9). */}
      {!isError && (isPending || posts.length === 0) && (
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

      <ul className="divide-divider mt-4 shrink-0 divide-y">
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
          // itself (spec/legacy/policy/posts.md) — the name gives way before the badge or the time do.
          const content = (
            <>
              <Typography
                variant="label"
                className="text-content-primary w-full truncate lg:w-auto lg:min-w-0 lg:flex-1"
              >
                {displayTitle(post)}
              </Typography>
              <span className="flex w-full min-w-0 items-center gap-2 lg:w-auto lg:shrink-0 lg:justify-end">
                <Badge tone={status.tone}>{status.label}</Badge>
                <VoiceRefLabel
                  voice={post.voice}
                  className={typographyStyles({ variant: 'meta' })}
                />
                {/* Only for an assigned post, and after the voice: the voice is on every row and
                    the 템플릿 is not, so it reads as an addition rather than a second column. */}
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
            'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 flex-col items-start justify-center gap-1 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:gap-4 lg:px-8'
          return (
            <li key={post.slug}>
              {post.pendingExperimentId && !post.activeJob ? (
                <Link
                  to="/ai-models/experiments/$id"
                  params={{ id: post.pendingExperimentId }}
                  className={rowClass}
                >
                  {content}
                </Link>
              ) : (
                <Link to="/posts/$slug" params={{ slug: post.slug }} className={rowClass}>
                  {content}
                </Link>
              )}
            </li>
          )
        })}
      </ul>

      {/* ONE 새 글, in the same place the voice directory puts its own add action. It used to be
          two — a docked bar on the phone and a second copy beside the heading from `sm:` up — which
          is two links to the same route in the DOM and two things to keep in step for a button
          that is never ambiguous about what it does. It docks in the thumb's band on a phone: in
          the top-right corner it was ~820px above the bottom edge of a 430x932 phone, a re-grip
          away from the one action this screen exists for (§4.3), and above the empty state that
          points at it. `mt-auto` puts it at the bottom of a SHORT list; `sticky` keeps it there
          once the list is long enough to scroll — at EVERY width, because a desk list scrolls too
          and the button went with it (THEME-24). Above the phone it shrinks to the width of the
          button and settles against the right edge. */}
      <ActionBar
        dock="list"
        ariaLabel={t('list.writingAria', { ns: 'posts' })}
        className="mx-4 mt-auto sm:mx-6 lg:mx-8"
      >
        <Link
          to="/posts/new"
          className={buttonStyles({ variant: 'cta', className: 'w-full sm:w-auto' })}
        >
          {t('new', { ns: 'posts' })}
        </Link>
      </ActionBar>
    </main>
  )
}
