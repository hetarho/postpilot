import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { displayTitle, postStatusLabel, usePosts, type PostListItem } from '@/entities/post'
import { useExperiments, type ModelExperiment } from '@/entities/model-experiment'
import { PurposeRefLabel } from '@/entities/purpose'
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

/** The way back to unfinished work (PRD F-8). The server returns only the acting user's
 *  posts, newest first — this screen does not sort or filter. */
export function PostsPage() {
  const { t } = useTranslation(['posts', 'common'])
  const { posts, isPending, isFetching, isError, refetch } = usePosts()
  const { experiments } = useExperiments()
  const byId = new Map(experiments.map((experiment) => [experiment.id, experiment]))

  return (
    // The page gutter lives on each block rather than on `main`, so the list rows can run edge to
    // edge: a pressed row that stops 16px short of the screen edge reads as a card, and a row inset
    // deeper than the page's own rhythm reads as a mistake (design-language §4.2).
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col py-6">
      {/* The CTA sits in TWO places because the two breakpoints want different shapes, and only
          one is ever rendered (the other is `display:none`, so it is absent from the a11y tree
          too). From `sm:` up the pointer is a mouse, §4.3's reach argument evaporates, and a bar
          docked to the bottom of a half-empty page reads as debris — so the action goes back
          beside the heading where a desktop user looks for it. */}
      <div className="flex items-center justify-between gap-3 px-4 sm:px-6">
        <Typography variant="display">{t('list.mine', { ns: 'posts' })}</Typography>
        <Link
          to="/posts/new"
          className={buttonStyles({ variant: 'cta', className: 'hidden sm:inline-flex' })}
        >
          {t('new', { ns: 'posts' })}
        </Link>
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
          className="text-content-tertiary mt-8 px-4 sm:px-6"
        >
          {isPending ? t('state.loading', { ns: 'common' }) : t('list.empty', { ns: 'posts' })}
        </Typography>
      )}

      <ul className="divide-divider mt-4 shrink-0 divide-y">
        {posts.map((post) => {
          const status = rowStatus(
            post,
            post.pendingExperimentId ? byId.get(post.pendingExperimentId) : undefined,
            t,
          )
          // Two lines, not three items competing on one. At 360px a single row left the title
          // ~146px — about ten Hangul — and because the badge label swings from 초안 to AI 결과 확인
          // the cut point moved row to row, so the list read as a ragged column of half-titles.
          // The voice sits between the status and the time as metadata: which voice a post is in
          // is the one thing this list newly has to say, and a tombstone must say so on the row
          // itself (spec/policy/posts.md) — the name gives way before the badge or the time do.
          const content = (
            <>
              <Typography variant="label" className="text-content-primary w-full truncate">
                {displayTitle(post)}
              </Typography>
              <span className="flex w-full min-w-0 items-center gap-2">
                <Badge tone={status.tone}>{status.label}</Badge>
                <VoiceRefLabel
                  voice={post.voice}
                  className={typographyStyles({ variant: 'meta' })}
                />
                {/* Only for an assigned post, and after the voice: the voice is on every row and
                    the 용도 is not, so it reads as an addition rather than a second column. */}
                <PurposeRefLabel
                  purpose={post.purpose}
                  className={typographyStyles({ variant: 'meta' })}
                />
                <time
                  dateTime={post.updatedAt}
                  className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}
                >
                  {formatRelativeTime(post.updatedAt)}
                </time>
              </span>
            </>
          )
          const rowClass =
            'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 flex-col items-start justify-center gap-1 px-4 py-3 sm:px-6'
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

      {/* The phone shape: the product's entry action docked in the thumb's band. In the top-right
          corner it was ~820px above the bottom edge of a 430x932 phone — a re-grip away from the
          one action this screen exists for (§4.3), and above the empty state that points at it.
          `mt-auto` is what puts it at the bottom of a SHORT list; `sticky` then keeps it there
          once the list is long enough to scroll. */}
      <ActionBar
        ariaLabel={t('list.writingAria', { ns: 'posts' })}
        className="mx-4 mt-auto sm:hidden"
      >
        <Link to="/posts/new" className={buttonStyles({ variant: 'cta', className: 'w-full' })}>
          {t('new', { ns: 'posts' })}
        </Link>
      </ActionBar>
    </main>
  )
}
