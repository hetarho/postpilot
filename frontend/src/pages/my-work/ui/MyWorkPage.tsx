import type { ComponentType, ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowRight, Clapperboard, Film, PenLine, Plus } from 'lucide-react'
import {
  displayTitle,
  isPostStatus,
  postStatusLabel,
  rememberPostEntry,
  usePostList,
  type PostListItem,
} from '@/entities/post'
import {
  clipChannel,
  clipDisplayTitle,
  clipFinished,
  clipState,
  clipHistoryStatus,
  rememberClipEntry,
  useClipProjects,
  type ClipProject,
} from '@/entities/clip-project'
import { isTerminal } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { formatRelativeTime } from '@/shared/lib'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
  type BadgeTone,
} from '@/shared/ui'
import {
  StatusTile,
  WorkGroup,
  WorkListToolbar,
  WorkListBody,
  WorkListLoading,
  workCardStyles,
  workCardLinkClass,
} from '@/widgets/work-list'
import { useMyWorkScrollReturn } from '../model/useMyWorkScrollReturn'

type Kind = 'post' | 'naverClip' | 'instaClip'
interface WorkItem {
  key: string
  kind: Kind
  title: string
  updatedAt: string
  status: { label: string; tone: BadgeTone }
  finished: boolean
  icon: ComponentType<{ className?: string }>
  id: string
  remember: () => void
}

/** The one link a work card is: the editor for a post, the workspace for a clip. */
function WorkLink({
  item,
  className,
  children,
}: {
  item: WorkItem
  className: string
  children: ReactNode
}) {
  return item.kind === 'post' ? (
    <Link
      to="/posts/$slug"
      params={{ slug: item.id }}
      onClick={item.remember}
      className={className}
    >
      {children}
    </Link>
  ) : (
    <Link
      to="/clips/$clipId"
      params={{ clipId: item.id }}
      onClick={item.remember}
      className={className}
    >
      {children}
    </Link>
  )
}

const POST_TONE: Record<string, BadgeTone> = {
  draft: 'neutral',
  review: 'accent',
  finalized: 'success',
  published: 'done',
}

/** Every work appears once in its counted group; the latest unfinished item is quietly emphasized. */
export function MyWorkPage() {
  const { t } = useTranslation(['channel', 'posts', 'clips', 'common', 'nav'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const posts = usePostList({})
  const clips = useClipProjects(ownerId)

  const postItem = (post: PostListItem): WorkItem => {
    const running = post.activeJob && !isTerminal(post.activeJob)
    const failed = !!post.latestOrdinaryFailure || post.activeJob?.status === 'failed'
    const finished = post.status === 'finalized' || post.status === 'published'
    return {
      key: `post:${post.slug}`,
      kind: 'post',
      title: displayTitle(post),
      updatedAt: post.updatedAt,
      status: running
        ? { label: t('list.state.generating', { ns: 'posts' }), tone: 'info' }
        : failed
          ? { label: t('list.state.failed', { ns: 'posts' }), tone: 'danger' }
          : {
              label: postStatusLabel(post.status),
              tone: isPostStatus(post.status) ? POST_TONE[post.status] : 'neutral',
            },
      finished: finished && !running,
      icon: PenLine,
      id: post.slug,
      // Opened from here, a post returns here with this scroll (THEME-58 →THEME-69).
      remember: () =>
        rememberPostEntry(ownerId, {
          path: '/my',
          section: 'mine',
          filters: {},
          scrollY: window.scrollY,
          targetId: post.slug,
        }),
    }
  }
  const clipItem = (project: ClipProject): WorkItem => {
    const channel = clipChannel(project.channel)
    const running = project.latestJob && !isTerminal(project.latestJob)
    return {
      key: `clip:${project.id}`,
      kind: channel === 'instagram_clip' ? 'instaClip' : 'naverClip',
      title: clipDisplayTitle(project),
      updatedAt: project.updatedAt,
      status: clipHistoryStatus(project),
      // 완성 holds a confirmed or finalized clip with no run going (CLIP-41).
      finished: clipFinished(clipState(project)) && !running,
      icon: channel === 'instagram_clip' ? Film : Clapperboard,
      id: project.id,
      remember: () =>
        rememberClipEntry(ownerId, {
          path: '/my',
          section: 'mine',
          scrollY: window.scrollY,
          targetId: project.id,
          filters: { channel },
        }),
    }
  }
  const items = [...posts.posts.map(postItem), ...(clips.data ?? []).map(clipItem)].sort((a, b) =>
    b.updatedAt.localeCompare(a.updatedAt),
  )
  const latest = items.find((item) => !item.finished)
  const inProgress = items.filter((item) => !item.finished)
  const done = items.filter((item) => item.finished)
  const pending = posts.isPending || clips.isPending
  const failed = posts.isError || clips.isError
  useMyWorkScrollReturn(ownerId, !pending && !failed)
  const card = (item: WorkItem) => (
    <li key={item.key} className={workCardStyles({ latest: item === latest })}>
      <WorkLink item={item} className={workCardLinkClass}>
        <StatusTile tone={item.status.tone} icon={item.icon} />
        <span className="flex min-w-0 flex-1 flex-col gap-1.5">
          <Typography variant="fieldTitle" as="span" className="line-clamp-2">
            {item.title}
          </Typography>
          <span className="flex min-w-0 flex-wrap items-center gap-2">
            <Badge tone={item.status.tone}>{item.status.label}</Badge>
            <span className={typographyStyles({ variant: 'meta' })}>
              {t(`work.kind.${item.kind}`)}
            </span>
            <time dateTime={item.updatedAt} className={typographyStyles({ variant: 'meta' })}>
              {formatRelativeTime(item.updatedAt)}
            </time>
          </span>
        </span>
        {item === latest && (
          <>
            <span className="sr-only">{t('work.continueAction')}</span>
            <ArrowRight
              aria-hidden="true"
              className="text-link-fg-current my-auto size-5 shrink-0"
            />
          </>
        )}
      </WorkLink>
    </li>
  )
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}>
      <h1 className="sr-only">{t('location.mine', { ns: 'nav' })}</h1>
      <WorkListToolbar />
      {failed && (
        <Notice tone="danger" role="alert" className="mt-4">
          <span>{t('list.loadFailed', { ns: 'posts' })}</span>
          <Button
            variant="ghost"
            onClick={() => {
              posts.refetch()
              void clips.refetch()
            }}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}
      {!failed && !pending && items.length === 0 && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-6">
          {t('work.empty')}
        </Typography>
      )}
      <WorkListBody>
        <WorkListLoading pending={pending} />
        {!pending && inProgress.length > 0 && (
          <WorkGroup heading={t('work.inProgress')} count={inProgress.length}>
            {inProgress.map(card)}
          </WorkGroup>
        )}
        {!pending && done.length > 0 && (
          <WorkGroup heading={t('work.done')} count={done.length}>
            {done.map(card)}
          </WorkGroup>
        )}
        {!pending && posts.hasNextPage && (
          <Link
            to="/naver-blog"
            className={buttonStyles({ variant: 'secondary', className: 'self-start' })}
          >
            {t('work.more', { name: t('work.kind.post') })}
          </Link>
        )}
      </WorkListBody>
      <ActionBar dock="list" className="action-list-inset mt-auto w-full sm:w-fit">
        <Link
          to="/create"
          className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full sm:w-auto' })}
        >
          <Plus aria-hidden="true" className="size-6 shrink-0" />
          {t('tabs.create', { ns: 'nav' })}
        </Link>
      </ActionBar>
    </main>
  )
}
