import type { TFunction } from 'i18next'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  clipState,
  clipStateLabel,
  useClipProjects,
  type ClipProject,
} from '@/entities/clip-project'
import { useClipTemplates } from '@/entities/clip-template'
import { isTerminal } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { ClipListControls, narrowClips, type ClipNarrowing } from '@/features/filter-clips'
import { appFailureFromConnect } from '@/shared/api'
import { formatRelativeTime } from '@/shared/lib'
import {
  ActionBar,
  AppFailureMessage,
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  typographyStyles,
  type BadgeTone,
} from '@/shared/ui'
import { pageStyles } from '@/shared/ui'

/** The one status chip a row carries. Colour never travels alone (§2.6): the tone only reinforces
 *  the label, so the label is chosen first and the tone follows it.
 *
 *  A running attempt and a failed one are things the JOB says, and they outrank the project's own
 *  state — a generation in flight is the most useful thing a row can tell you. Everything else is
 *  `clipState`, the same function the workspace's step bar and status line read (CLIP-41). */
function rowStatus(
  project: ClipProject,
  t: TFunction<'clips'>,
): { label: string; tone: BadgeTone } {
  if (project.latestJob && !isTerminal(project.latestJob))
    return { label: t('state.generating'), tone: 'info' }
  if (project.latestJob?.status === 'failed') return { label: t('state.failed'), tone: 'danger' }
  const state = clipState(project)
  return { label: clipStateLabel(state), tone: stateTone(state) }
}

/** 초안 stays neutral (nothing has happened yet), 다듬는 중 takes the accent (the owner is
 *  mid-way), 완성 takes success. The label still carries the meaning on its own. */
function stateTone(state: ReturnType<typeof clipState>): BadgeTone {
  if (state === 'refining') return 'accent'
  if (state === 'finished') return 'success'
  return 'neutral'
}

/** The way back to unfinished clips. The server returns only the acting user's projects (CLIP-2);
 *  the search and the status filter narrow that one answer here in the browser, reading and
 *  writing the URL so the narrowing survives opening a project, coming back, a reload and a
 *  shared link (CLIP-41). */
export function ClipsPage() {
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const projects = useClipProjects(user?.id ?? '')
  const templates = useClipTemplates(user?.id ?? '')
  const narrowing: ClipNarrowing = useSearch({ strict: false })
  const navigate = useNavigate()
  // `replace`, not a push: a history entry per keystroke would make 뒤로 mean "one character ago"
  // instead of "the screen I came from". An emptied field drops the param rather than carrying `?q=`.
  const narrow = (next: ClipNarrowing) =>
    void navigate({
      to: '/clips',
      search: { q: next.q?.trim() === '' ? undefined : next.q, status: next.status },
      replace: true,
    })
  const listed = projects.data ?? []
  const narrowed = narrowClips(listed, narrowing)
  const noMatch = !projects.isPending && !projects.isError && listed.length > 0 && !narrowed.length
  const statusLabel = narrowing.status ? clipStateLabel(narrowing.status) : ''
  const noMatchText = narrowing.q?.trim()
    ? narrowing.status
      ? t('project.noMatchBoth', { ns: 'clips', q: narrowing.q.trim(), status: statusLabel })
      : t('project.noMatchQuery', { ns: 'clips', q: narrowing.q.trim() })
    : t('project.noMatchStatus', { ns: 'clips', status: statusLabel })

  return (
    // The page gutter lives on each block rather than on `main`, so the rows can run edge to edge:
    // a pressed row that stops 16px short of the screen edge reads as a card (§4.2).
    <main
      className={pageStyles({ width: 'wide', gutters: false, className: 'flex flex-1 flex-col' })}
    >
      <div className="px-4 sm:px-6 lg:px-8">
        <Typography variant="display">{t('title', { ns: 'clips' })}</Typography>
      </div>

      {/* On the screen at every project count: a search that appears at some number of projects
          is a second layout for the same page. */}
      <div className="mt-6 px-4 sm:px-6 lg:px-8">
        <ClipListControls narrowing={narrowing} onChange={narrow} />
      </div>

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

      {/* One live region for both states, so finishing the load is a text change inside it rather
          than two nodes swapping — a swap announces nothing to a screen reader. */}
      {!projects.isError && (projects.isPending || listed.length === 0) && (
        <Typography
          variant="body"
          role="status"
          className="text-content-tertiary mt-8 px-4 sm:px-6 lg:px-8"
        >
          {projects.isPending
            ? t('state.loading', { ns: 'common' })
            : t('project.empty', { ns: 'clips' })}
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

      <ul
        aria-label={t('project.directory', { ns: 'clips' })}
        className="divide-divider mt-4 shrink-0 divide-y"
      >
        {narrowed.map((project) => {
          const status = rowStatus(project, t)
          const template = templates.templates.find((v) => v.id === project.videoTemplateId)
          // Two stacked lines on a phone, one line on the desk — the same rule the post list
          // follows: at 360px a single row leaves the title about ten Hangul, and from `lg:` up
          // the same two lines read as a ragged double-height list with the right two thirds empty.
          return (
            <li key={project.id}>
              <Link
                to="/clips/$clipId"
                params={{ clipId: project.id }}
                className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 flex-col items-start justify-center gap-1 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:gap-4 lg:px-8"
              >
                <Typography
                  variant="label"
                  className="text-content-primary w-full truncate lg:w-auto lg:min-w-0 lg:flex-1"
                >
                  {project.title}
                </Typography>
                <span className="flex w-full min-w-0 items-center gap-2 lg:w-auto lg:shrink-0 lg:justify-end">
                  <Badge tone={status.tone}>{status.label}</Badge>
                  <span className={typographyStyles({ variant: 'meta', className: 'truncate' })}>
                    {template?.name ??
                      t(
                        project.videoTemplateId && templates.isPending
                          ? 'project.templateLoading'
                          : project.videoTemplateId && templates.isError
                            ? 'project.templateUnavailable'
                            : 'project.detachedTemplate',
                        { ns: 'clips' },
                      )}
                  </span>
                  <span className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}>
                    {t(`ratio.${project.ratio}`, { ns: 'clips' })}
                  </span>
                  <time
                    dateTime={project.updatedAt}
                    className={typographyStyles({ variant: 'meta', className: 'shrink-0' })}
                  >
                    {formatRelativeTime(project.updatedAt)}
                  </time>
                </span>
              </Link>
            </li>
          )
        })}
      </ul>

      {/* ONE 새 클립, docked in the thumb's band on a phone and shrunk to the button's own width
          above it (THEME-24). `mt-auto` puts it at the bottom of a SHORT list; `sticky` keeps it
          there once the list is long enough to scroll. */}
      <ActionBar
        dock="list"
        ariaLabel={t('project.newDockAria', { ns: 'clips' })}
        className="mx-4 mt-auto sm:mx-6 lg:mx-8"
      >
        <Link
          to="/clips/new"
          className={buttonStyles({ variant: 'cta', className: 'w-full sm:w-auto' })}
        >
          {t('project.new', { ns: 'clips' })}
        </Link>
      </ActionBar>
    </main>
  )
}
