import { useState, type ComponentType, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowRight, Clapperboard, PenLine, Plus } from 'lucide-react'
import {
  displayTitle,
  postRowState,
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
  ListControls,
  Notice,
  SegmentedControl,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
  type BadgeTone,
} from '@/shared/ui'
import {
  FindSheet,
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
type Sort = 'updated' | 'oldest' | 'title'
type StatusFilter = 'all' | 'inProgress' | 'done'
interface Narrowing {
  q: string
  kind: Kind | 'all'
  status: StatusFilter
  sort: Sort
}
const NO_NARROWING: Narrowing = { q: '', kind: 'all', status: 'all', sort: 'updated' }
const KINDS: readonly (Kind | 'all')[] = ['all', 'post', 'naverClip']
const SORTS: readonly Sort[] = ['updated', 'oldest', 'title']
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

/** Every work appears once in its counted group; the latest unfinished item is quietly emphasized. */
export function MyWorkPage() {
  const { t } = useTranslation(['channel', 'posts', 'clips', 'common', 'nav'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const posts = usePostList({})
  const clips = useClipProjects(ownerId)
  const [narrowing, setNarrowing] = useState<Narrowing>(NO_NARROWING)
  const narrow = (patch: Partial<Narrowing>) =>
    setNarrowing((current) => ({ ...current, ...patch }))

  const postItem = (post: PostListItem): WorkItem => {
    const running = post.activeJob && !isTerminal(post.activeJob)
    const finished = post.status === 'finalized' || post.status === 'published'
    return {
      key: `post:${post.slug}`,
      kind: 'post',
      title: displayTitle(post),
      updatedAt: post.updatedAt,
      // The blog list's chip, with its precedence (POST-127); this page reads no test status.
      status: postRowState(post),
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
      // One 영상 kind, whichever channel the video was made for.
      kind: 'naverClip',
      title: clipDisplayTitle(project),
      updatedAt: project.updatedAt,
      status: clipHistoryStatus(project),
      // 완성 holds a confirmed or finalized clip with no run going (CLIP-41).
      finished: clipFinished(clipState(project)) && !running,
      icon: Clapperboard,
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
  const all = [...posts.posts.map(postItem), ...(clips.data ?? []).map(clipItem)]
  // 찾기: a title search, the kind and the status narrow the one loaded answer; the order is the
  // owner's choice, newest change first by default.
  const query = narrowing.q.trim().toLocaleLowerCase()
  const items = all
    .filter(
      (item) =>
        (!query || item.title.toLocaleLowerCase().includes(query)) &&
        (narrowing.kind === 'all' || item.kind === narrowing.kind) &&
        (narrowing.status === 'all' || item.finished === (narrowing.status === 'done')),
    )
    .sort((a, b) =>
      narrowing.sort === 'title'
        ? a.title.localeCompare(b.title)
        : narrowing.sort === 'oldest'
          ? a.updatedAt.localeCompare(b.updatedAt)
          : b.updatedAt.localeCompare(a.updatedAt),
    )
  const active =
    (query ? 1 : 0) + (narrowing.kind !== 'all' ? 1 : 0) + (narrowing.status !== 'all' ? 1 : 0)
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
      <WorkListToolbar summary={t(`work.sort.${narrowing.sort}`)}>
        <FindSheet
          active={active}
          onClear={() => setNarrowing((current) => ({ ...NO_NARROWING, sort: current.sort }))}
        >
          <div className="space-y-5">
            <ListControls<Kind | 'all'>
              searchLabel={t('work.search')}
              searchPlaceholder={t('work.searchPlaceholder')}
              query={narrowing.q}
              onQueryChange={(q) => narrow({ q })}
              filter={{
                value: narrowing.kind,
                options: KINDS.map((value) => ({
                  value,
                  label: value === 'all' ? t('work.kindFilter.all') : t(`work.kind.${value}`),
                })),
                onChange: (kind) => narrow({ kind }),
                label: t('work.kindFilter.label'),
              }}
            />
            <div>
              <Typography variant="label" as="p">
                {t('work.statusFilter.label')}
              </Typography>
              <SegmentedControl<StatusFilter>
                value={narrowing.status}
                ariaLabel={t('work.statusFilter.label')}
                options={(['all', 'inProgress', 'done'] as const).map((value) => ({
                  value,
                  label: t(`work.statusFilter.${value}`),
                }))}
                onChange={(status) => narrow({ status })}
                className="mt-1"
              />
            </div>
            <div>
              <Typography variant="label" as="p">
                {t('work.sort.label')}
              </Typography>
              <SegmentedControl<Sort>
                value={narrowing.sort}
                ariaLabel={t('work.sort.label')}
                options={SORTS.map((value) => ({ value, label: t(`work.sort.${value}`) }))}
                onChange={(sort) => narrow({ sort })}
                className="mt-1"
              />
            </div>
          </div>
        </FindSheet>
      </WorkListToolbar>
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
          {t(all.length ? 'work.noMatch' : 'work.empty')}
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
