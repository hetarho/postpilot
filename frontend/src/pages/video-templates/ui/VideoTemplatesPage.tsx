import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipTemplates, type ClipTemplate } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { DeleteClipTemplateButton } from '@/features/delete-clip-template'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** The video-template directory, in the shape the post-template directory already has (CLIP-42):
 *  the same rows, the same usage badge, the same failure and empty treatments, and one docked CTA. */
export function VideoTemplatesPage() {
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isFetching, isError, refetch } = useClipTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'video-template' })

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Typography variant="display">{t('directory.title', { ns: 'clips' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('directory.description', { ns: 'clips' })}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('directory.loadFailed', { ns: 'clips' })}</span>
          {/* `isFetching`, not `isPending`: react-query keeps `status: 'error'` across a refetch,
              so without it the notice does not move for the seconds a retry takes on cellular. */}
          <Button
            variant="ghost"
            onClick={() => void refetch()}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!isError && !isPending && (
        <>
          {summaries.isPending && (
            <Typography variant="body" role="status" className="mt-6">
              {t('directory.checkingWork', { ns: 'clips' })}
            </Typography>
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('directory.workLoadFailed', { ns: 'clips' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="video-templates-heading" className="mt-8">
              <Typography variant="title" id="video-templates-heading">
                {t('directory.saved', { ns: 'clips' })}
              </Typography>
              {/* Rows are full-bleed against the page gutter, so the list cancels it (THEME-23). */}
              <ul className="divide-divider -mx-4 mt-3 divide-y sm:-mx-6 lg:-mx-8">
                {templates.map((template) => (
                  <TemplateRow
                    key={template.id}
                    ownerId={ownerId}
                    template={template}
                    summary={summaries.data?.find(
                      (row) =>
                        row.targetId === template.id || row.lastPublication?.id === template.id,
                    )}
                  />
                ))}
              </ul>
            </section>
          )}
          {summaries.data?.some((row) => !row.targetId && !row.lastPublication) && (
            <section className="mt-8" aria-labelledby="video-templates-unsaved-heading">
              <Typography variant="title" as="h2" id="video-templates-unsaved-heading">
                {t('directory.unsavedWork', { ns: 'clips' })}
              </Typography>
              <ul className="mt-3 space-y-3">
                {summaries.data
                  .filter((row) => !row.targetId && !row.lastPublication)
                  .map((row) => (
                    <li key={row.sessionId}>
                      <Link
                        to="/video-templates/new"
                        search={(previous) => ({ ...previous, session: row.sessionId })}
                        className={typographyStyles({
                          variant: 'body',
                          className: 'text-link-fg inline-flex min-h-11 items-center underline',
                        })}
                      >
                        {t('directory.resumeNew', {
                          ns: 'clips',
                          name: row.displayName || t('directory.create', { ns: 'clips' }),
                        })}
                      </Link>
                      <Typography variant="body" className="text-content-secondary">
                        {t('directory.unsaved', { ns: 'clips' })}
                      </Typography>
                      <EditingStatus summary={row} />
                    </li>
                  ))}
              </ul>
            </section>
          )}

          {/* One instance, docked at every width: the thumb is why it is full-bleed on a phone,
              and a list that scrolls is why it stays docked above one (THEME-24). */}
          <ActionBar
            dock="list"
            ariaLabel={t('directory.newDockAria', { ns: 'clips' })}
            className="mt-auto"
          >
            <Link to="/video-templates/new" className={buttonStyles({ variant: 'cta' })}>
              {t('directory.create', { ns: 'clips' })}
            </Link>
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** Plain language and no example: the empty state says what a video template is FOR and lets the
 *  editor teach its own vocabulary. It creates nothing — there are no shipped presets. */
function EmptyState() {
  const { t } = useTranslation('clips')
  return (
    <section aria-labelledby="video-templates-empty-heading" className="mt-8">
      <Typography variant="title" id="video-templates-empty-heading">
        {t('directory.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('directory.emptyHelp')}
      </Typography>
    </section>
  )
}

/** One template, one target. The link stretches over the whole row through its `::after`, so the
 *  padding and the empty space navigate too, while the delete paints above that layer and acts
 *  without navigating — a row is one target, not a row with a button inside it (THEME-23). */
function TemplateRow({
  ownerId,
  template,
  summary,
}: {
  ownerId: string
  template: ClipTemplate
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation('clips')
  return (
    // `min-h-16` and `py-2`, not the list row's usual `min-h-11`/`py-3`: every row carries the
    // delete, which keeps the 44px floor, so the row is 44 plus its own padding (THEME-23).
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2 sm:px-6 lg:px-8">
      <Link
        to="/video-templates/$templateId"
        params={{ templateId: template.id }}
        className={typographyStyles({
          variant: 'label',
          className: 'min-w-0 truncate after:absolute after:inset-0',
        })}
      >
        {template.name}
      </Link>
      <div className="basis-full">
        <Typography variant="body" className="text-content-secondary">
          {t('directory.savedAvailable')}
        </Typography>
        <EditingStatus summary={summary} />
      </div>
      <div className="relative ml-auto flex shrink-0 items-center gap-2">
        <Badge tone="neutral">
          {t('directory.projectCount', { count: template.projectCount })}
        </Badge>
        <DeleteClipTemplateButton ownerId={ownerId} template={template} />
      </div>
    </li>
  )
}
function EditingStatus({ summary }: { summary?: AuthoringSummary }) {
  const { t } = useTranslation(['clips', 'authoring'])
  if (!summary) return null
  const named = {
    kind: t('kinds.video-template', { ns: 'authoring' }),
    name: summary.displayName || t('directory.create', { ns: 'clips' }),
  }
  return (
    <div role="status" className="mt-1 space-y-1">
      {summary.lastPublication?.outcome && (
        <Typography variant="body">
          {t('directory.lastPublication', {
            ns: 'clips',
            result: t(
              summary.lastPublication.outcome === 'created' ? 'confirmedCreate' : 'confirmedUpdate',
              { ns: 'authoring', kind: named.kind, name: summary.lastPublication.name },
            ),
          })}
        </Typography>
      )}
      {summary.hasUnpublishedChanges && (
        <Typography variant="body">{t('unpublished', { ns: 'authoring', ...named })}</Typography>
      )}
      {summary.activeJobId && (
        <Typography variant="body">
          {t('directory.activeAI', { ns: 'clips', name: named.name })}
        </Typography>
      )}
      {summary.publicationPending && (
        <Typography variant="body">
          {t('directory.publicationPending', { ns: 'clips', name: named.name })}
        </Typography>
      )}
      {summary.targetConflict && (
        <Typography variant="body">
          {t('directory.conflict', { ns: 'clips', name: named.name })}
        </Typography>
      )}
    </div>
  )
}
