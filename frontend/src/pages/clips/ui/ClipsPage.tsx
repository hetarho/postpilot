import { useEffect } from 'react'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  clipState,
  clipChannel,
  clipChannelPath,
  clipHistoryStateLabel,
  clipHistoryStatus,
  type ClipProject,
  useClipProjects,
  rememberClipEntry,
  readClipHistoryReturn,
  completeClipHistoryReturn,
  type ClipChannel,
} from '@/entities/clip-project'
import { useClipTemplates } from '@/entities/clip-template'
import { isTerminal, progressLabel } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { ClipListControls, narrowClips, type ClipNarrowing } from '@/features/filter-clips'
import { Clapperboard, Film } from 'lucide-react'
import { ChannelActions } from '@/widgets/channel-actions'
import { clsx } from 'clsx'
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
import { appFailureFromConnect } from '@/shared/api'
import { activeLocale, formatAppFailure, formatRelativeTime } from '@/shared/lib'
import {
  AppFailureMessage,
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  typographyStyles,
} from '@/shared/ui'
import { pageStyles } from '@/shared/ui'
import { i18n } from '../config/i18n'

/** A clip channel's own page: the way back to its unfinished clips (CLIP-3, CLIP-41). The server
 *  returns only the acting user's projects (CLIP-2); the channel comes from the address, and the
 *  search and the status filter narrow that one answer here in the browser, reading and writing
 *  the URL so the narrowing survives opening a project, coming back, a reload and a shared link. */
function ClipsPage({ channel }: { channel: ClipChannel }) {
  const { t } = useTranslation(['clips', 'common'])
  // The app registers the fragment centrally. Slice defaults keep the
  // history usable before that wiring lands and still follow the selected UI language.
  const historyText = (key: keyof typeof i18n.ko.history) =>
    t(`history.${key}`, { ns: 'clips', defaultValue: i18n[activeLocale()].history[key] })
  const { user } = useSession()
  const projects = useClipProjects(user?.id ?? '')
  const templates = useClipTemplates(user?.id ?? '')
  const search: Omit<ClipNarrowing, 'channel'> = useSearch({ strict: false })
  const narrowing: ClipNarrowing = { q: search.q, status: search.status, channel }
  const path = clipChannelPath(channel)
  const navigate = useNavigate()
  useEffect(() => {
    if (projects.isPending || projects.isError || !user?.id) return
    const origin = readClipHistoryReturn(user.id)
    if (!origin) return
    const frame = window.requestAnimationFrame(() => {
      window.scrollTo(0, origin.scrollY)
      completeClipHistoryReturn(user.id)
    })
    return () => window.cancelAnimationFrame(frame)
  }, [projects.isPending, projects.isError, user?.id])
  const remember = (targetId: string) => {
    if (!user?.id) return
    rememberClipEntry(user.id, {
      path,
      section: 'clips',
      scrollY: window.scrollY,
      targetId,
      filters: {
        channel,
        ...(narrowing.q ? { q: narrowing.q } : {}),
        ...(narrowing.status ? { status: narrowing.status } : {}),
      },
    })
  }
  // `replace`, not a push: a history entry per keystroke would make 뒤로 mean "one character ago"
  // instead of "the screen I came from". An emptied field drops the param rather than carrying `?q=`.
  const narrow = (next: ClipNarrowing) =>
    void navigate({
      to: path,
      search: {
        q: next.q?.trim() === '' ? undefined : next.q,
        status: next.status,
      },
      replace: true,
    })
  const listed = (projects.data ?? []).filter((project) => clipChannel(project.channel) === channel)
  const narrowed = narrowClips(listed, narrowing)
  // Guide: 완성 is a finalized clip with no run going; everything else is 작성 중. The latest
  // unfinished clip is emphasized inside that group unless a narrowing is on.
  const finished = (project: ClipProject) =>
    clipState(project) === 'finished' && !(project.latestJob && !isTerminal(project.latestJob))
  const isNarrowed = (narrowing.q ?? '').trim() !== '' || narrowing.status !== undefined
  // The server lists the projects newest first, so the first unfinished one is the newest.
  const latest = isNarrowed ? undefined : narrowed.find((project) => !finished(project))
  const groups = [
    {
      key: 'inProgress',
      items: narrowed.filter((project) => !finished(project)),
    },
    { key: 'done', items: narrowed.filter(finished) },
  ] as const
  const noMatch = !projects.isPending && !projects.isError && listed.length > 0 && !narrowed.length
  const statusLabel = narrowing.status ? clipHistoryStateLabel(narrowing.status) : ''
  const noMatchText = narrowing.q?.trim()
    ? narrowing.status
      ? t('project.noMatchBoth', { ns: 'clips', q: narrowing.q.trim(), status: statusLabel })
      : t('project.noMatchQuery', { ns: 'clips', q: narrowing.q.trim() })
    : t('project.noMatchStatus', { ns: 'clips', status: statusLabel })

  return (
    // The page gutter lives on each block rather than on `main`, so the rows can run edge to edge:
    // a pressed row that stops 16px short of the screen edge reads as a card (THEME-23).
    <main
      className={pageStyles({
        width: 'wide',
        gutters: false,
        className: 'flex flex-1 flex-col pt-2 sm:pt-4',
      })}
    >
      {/* The channel opens straight onto this list: the marked channel link names the place, so
          the heading is for assistive technology only (THEME-38, CLIP-41). */}
      <h1 className="sr-only">
        {historyText(channel === 'instagram_clip' ? 'instaClip' : 'naverClip')}
      </h1>

      {/* Guide: the search and the status filter wait behind ONE 찾기 button (a bottom sheet), so
          the list opens on its work. */}
      <WorkListToolbar className="px-4 sm:px-6 lg:px-8">
        <FindSheet
          active={(narrowing.q?.trim() ? 1 : 0) + (narrowing.status ? 1 : 0)}
          onClear={() => narrow({ channel })}
        >
          <ClipListControls narrowing={narrowing} onChange={narrow} />
        </FindSheet>
      </WorkListToolbar>

      {templates.isError && !projects.isError && (
        <Notice tone="danger" role="alert" className="mx-4 mt-8 sm:mx-6">
          <AppFailureMessage failure={appFailureFromConnect(templates.error)} />
          <Button
            variant="ghost"
            onClick={() => void templates.refetch()}
            pending={templates.isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}

      {projects.isError && (
        <Notice tone="danger" role="alert" className="mx-4 mt-8 sm:mx-6">
          <AppFailureMessage failure={appFailureFromConnect(projects.error)} />
          {/* `isFetching`, not `isPending`: react-query keeps `status: 'error'` across a refetch,
              so without it the notice does not move for the several seconds a retry takes on
              cellular and the owner taps it again and again. */}
          <Button
            variant="ghost"
            onClick={() => void projects.refetch()}
            pending={projects.isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}

      {/* Empty-account feedback; the shared list body owns loading geometry and its live region. */}
      {!projects.isError && !projects.isPending && listed.length === 0 && (
        <Typography
          variant="body"
          role="status"
          className="text-content-tertiary mt-8 px-4 sm:px-6 lg:px-8"
        >
          {historyText('empty')}
        </Typography>
      )}

      {/* A narrowing that matches nothing is NOT the same screen as an account with no clips: it
          names what is narrowing, so the owner can see it is their own query and not an empty
          account, and it offers the one way back to the whole list. */}
      {noMatch && (
        <div className="mt-8 px-4 sm:px-6 lg:px-8">
          <Typography variant="body" role="status" className="text-content-tertiary">
            {noMatchText}
          </Typography>
          <Button variant="ghost" onClick={() => narrow({})} className="mt-2 -ml-3">
            {t('project.reset', { ns: 'clips' })}
          </Button>
        </div>
      )}

      {/* Every item appears once; latest-item emphasis never adds another section or density. */}
      <WorkListBody
        label={t('project.directory', { ns: 'clips' })}
        className="px-4 sm:px-6 lg:px-8"
      >
        <WorkListLoading pending={projects.isPending} />
        {(() => {
          // One clip's card: a status tile, the title, its state, template, ratio and time, and
          // the recovery links under it. Continuation stays a compact link on every card.
          const renderClip = (project: ClipProject, latestCard = false) => {
            const status = clipHistoryStatus(project)
            const template = templates.templates.find((v) => v.id === project.videoTemplateId)
            const runningJob =
              project.latestJob && !isTerminal(project.latestJob) ? project.latestJob : undefined
            const failure =
              project.latestJob?.status === 'failed' ? project.latestJob.failure : undefined
            // Guide: a large, friendly card — the status tile, the title in a heading's size, then
            // its state and metadata wrapping under it.
            return (
              <li key={project.id} className={workCardStyles({ latest: latestCard })}>
                <Link
                  to="/clips/$clipId"
                  params={{ clipId: project.id }}
                  onClick={() => remember(project.id)}
                  className={workCardLinkClass}
                >
                  <StatusTile
                    tone={status.tone}
                    icon={channel === 'instagram_clip' ? Film : Clapperboard}
                  />
                  <span className="flex min-w-0 flex-1 flex-col gap-2">
                    <Typography
                      variant="fieldTitle"
                      as="span"
                      className="text-content-primary line-clamp-2"
                    >
                      {project.title.trim() || historyText('untitled')}
                    </Typography>
                    <span className="flex w-full min-w-0 flex-wrap items-center gap-2">
                      <Badge tone={status.tone}>{status.label}</Badge>
                      <span
                        className={typographyStyles({ variant: 'meta', className: 'truncate' })}
                      >
                        {template?.name ??
                          t(
                            // No template is a normal state and asks for nothing;
                            // only a NAMED template that is gone does (CLIP-146).
                            !project.videoTemplateId
                              ? 'project.noTemplate'
                              : templates.isPending
                                ? 'project.templateLoading'
                                : templates.isError
                                  ? 'project.templateUnavailable'
                                  : 'project.detachedTemplate',
                            { ns: 'clips' },
                          )}
                      </span>
                      <span
                        className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}
                      >
                        {t(`ratio.${project.ratio}`, { ns: 'clips' })}
                      </span>
                      <time
                        dateTime={project.updatedAt}
                        className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}
                      >
                        {formatRelativeTime(project.updatedAt)}
                      </time>
                    </span>
                  </span>
                </Link>
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
                    to="/clips/$clipId"
                    params={{ clipId: project.id }}
                    onClick={() => remember(project.id)}
                    className={buttonStyles({
                      variant: 'ghost',
                      className: clsx(
                        '-ml-3',
                        workCardActionClass,
                        latestCard && 'text-link-fg-current',
                      ),
                    })}
                  >
                    {historyText(project.finalized || runningJob ? 'open' : 'continue')}
                  </Link>
                  {project.result?.downloadUrl ? (
                    <a
                      href={project.result.downloadUrl}
                      className={buttonStyles({ variant: 'ghost', className: workCardActionClass })}
                    >
                      {historyText('download')}
                    </a>
                  ) : project.result ? (
                    <Link
                      to="/clips/$clipId"
                      params={{ clipId: project.id }}
                      onClick={() => remember(project.id)}
                      className={buttonStyles({ variant: 'ghost', className: workCardActionClass })}
                    >
                      {historyText('export')}
                    </Link>
                  ) : null}
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
                      {group.items.map((item) => renderClip(item, item === latest))}
                    </WorkGroup>
                  ),
              )}
            </>
          )
        })()}
      </WorkListBody>
      <ChannelActions channel={channel} section="work" className="action-list-inset mt-auto" />
    </main>
  )
}

export function NaverClipHistoryPage() {
  return <ClipsPage channel="naver_clip" />
}
export function InstaClipHistoryPage() {
  return <ClipsPage channel="instagram_clip" />
}
