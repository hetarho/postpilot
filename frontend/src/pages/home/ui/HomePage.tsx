import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { PenLine } from 'lucide-react'
import { isTerminal } from '@/entities/generation-job'
import {
  displayTitle,
  POST_LIST_PATH,
  rememberPostEntry,
  usePostList,
  type PostListItem,
} from '@/entities/post'
import { useSession } from '@/entities/session'
import { WritingKit, useWritingKit } from '@/widgets/writing-kit'
import { ActionBar, Typography, buttonStyles, pageStyles, typographyStyles } from '@/shared/ui'

/** Guide: 홈 — the product's promise, and whether the owner's kit is ready to keep it. The
 *  promise is the screen's one message; under it, the kit drawn as the post it makes (말투 ·
 *  템플릿 · 지침 · 기억 · 쓰는 AI, and 목소리 for videos), each part a way to the screen that makes it
 *  ready; then the one 글쓰기 in the thumb zone. Nothing here starts paid work: 글쓰기 opens the
 *  editor, which asks before it spends. */
export function HomePage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { parts, summary } = useWritingKit(ownerId)
  const drafts = usePostList({ q: '' })
  const latest = drafts.posts.find(unfinished)
  const phaseLine =
    summary.phase === 'loading'
      ? ''
      : summary.phase === 'ready'
        ? t('home.phase.ready')
        : summary.phase === 'empty'
          ? t('home.phase.empty')
          : summary.next
            ? t(`home.phase.next.${summary.next as 'voice'}`)
            : ''
  // The new post keeps the 글 list as its way back — it is where the post will be found.
  const remember = (targetId?: string) =>
    rememberPostEntry(ownerId, {
      path: POST_LIST_PATH,
      section: 'post',
      filters: {},
      scrollY: 0,
      ...(targetId ? { targetId } : {}),
    })
  return (
    <main
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-4 sm:pt-10' })}
    >
      {/* On the desk the promise and the kit stand side by side, so the whole kit is in view
          above the dock; on a phone they stack. */}
      <div className="lg:grid lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)] lg:items-center lg:gap-12">
        <div className="mx-auto flex w-full max-w-2xl flex-col items-center text-center lg:items-start lg:text-left">
          <p
            className={typographyStyles({ variant: 'label', className: 'tabular-nums' })}
            aria-live="polite"
          >
            {summary.settled
              ? t('home.eyebrow', { ready: summary.ready, total: summary.total })
              : ''}
            &nbsp;
          </p>
          <h1
            className={typographyStyles({ variant: 'largeTitle', className: 'mt-1 text-balance' })}
          >
            {t('home.promiseLead')}{' '}
            <span className="text-badge-accent-fg">{t('home.promiseAccent')}</span>
          </h1>
          <Typography
            variant="body"
            role="status"
            className="text-content-secondary max-w-measure mt-3 min-h-[1lh] text-balance"
          >
            {phaseLine}
          </Typography>
        </div>

        <WritingKit
          parts={parts}
          summary={summary}
          className="animate-step-in mx-auto mt-6 max-w-3xl sm:mt-10 lg:mt-0"
        />
      </div>

      <ActionBar
        dock="list"
        ariaLabel={t('home.actions')}
        className="mt-auto flex w-full min-w-0 flex-wrap gap-2 sm:w-fit"
      >
        {latest && (
          <Link
            to="/posts/$slug"
            params={{ slug: latest.slug }}
            onClick={() => remember(latest.slug)}
            aria-label={t('home.resumeNamed', { title: displayTitle(latest) })}
            className={buttonStyles({
              variant: 'secondary',
              size: 'hero',
              className: 'min-w-0 flex-1 basis-40 sm:flex-none sm:basis-auto',
            })}
          >
            <span className="truncate">{t('home.resume')}</span>
          </Link>
        )}
        <Link
          to="/posts/new"
          search={{}}
          onClick={() => remember()}
          className={buttonStyles({
            variant: 'cta',
            size: 'hero',
            className: 'min-w-0 flex-[2] basis-40 sm:flex-none sm:basis-auto',
          })}
        >
          <PenLine aria-hidden="true" className="size-6 shrink-0" />
          {t('home.write')}
        </Link>
      </ActionBar>
    </main>
  )
}

/** Still being written: not 확정 or 발행, or with a run going. */
function unfinished(post: PostListItem): boolean {
  return (
    (post.status !== 'finalized' && post.status !== 'published') ||
    !!(post.activeJob && !isTerminal(post.activeJob))
  )
}
