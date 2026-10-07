import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { useTemplates, type Template } from '@/entities/template'
import { DeleteTemplateButton } from '@/features/delete-template'
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

/** The account's 템플릿 (TMPL-24): a list, and the one action that adds to it. Composition only —
 *  every action is its own feature, and a row is the way into one template.
 *
 *  Editing lives on the template's own screen, so this list carries nothing but the list (change
 *  30). Nothing here calls a model or enqueues a job: a template is authored text, and reading or
 *  deleting one is a plain CRUD round trip ([I5]). */
export function TemplatesPage() {
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const summaries = useAuthoringSummaries({ ownerId, kind: 'post-template' })

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Typography variant="display">{t('title', { ns: 'templates' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.description', { ns: 'templates' })}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed', { ns: 'templates' })}</span>
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
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!isError && !isPending && (
        <>
          {summaries.isPending && (
            <Typography variant="body" role="status" className="mt-6">
              {t('screen.checkingWork', { ns: 'templates' })}
            </Typography>
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('screen.workLoadFailed', { ns: 'templates' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="templates-heading" className="mt-8">
              <Typography variant="title" id="templates-heading">
                {t('page.saved', { ns: 'templates' })}
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
            <section className="mt-8" aria-labelledby="templates-unsaved-heading">
              <Typography variant="title" as="h2" id="templates-unsaved-heading">
                {t('screen.unsavedWork', { ns: 'templates' })}
              </Typography>
              <ul className="mt-3 space-y-3">
                {summaries.data
                  .filter((row) => !row.targetId && !row.lastPublication)
                  .map((row) => (
                    <li key={row.sessionId}>
                      <Link
                        to="/templates/new"
                        search={{ session: row.sessionId }}
                        className={typographyStyles({
                          variant: 'body',
                          className: 'text-link-fg inline-flex min-h-11 items-center underline',
                        })}
                      >
                        {t('screen.resumeNew', {
                          ns: 'templates',
                          name: row.displayName || t('page.new', { ns: 'templates' }),
                        })}
                      </Link>
                      <Typography variant="body" className="text-content-secondary">
                        {t('screen.unsaved', { ns: 'templates' })}
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
            ariaLabel={t('page.newDockAria', { ns: 'templates' })}
            className="mt-auto"
          >
            <Link to="/templates/new" className={buttonStyles({ variant: 'cta' })}>
              {t('page.new', { ns: 'templates' })}
            </Link>
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** Plain language and no example. The grammar is what the write prompt reads, not something the
 *  user authors in — so the empty state says what a template is FOR and lets the composition
 *  editor teach its own vocabulary (TMPL-24). It creates nothing: there are no shipped presets. */
function EmptyState() {
  const { t } = useTranslation('templates')
  return (
    <section aria-labelledby="templates-empty-heading" className="mt-8">
      <Typography variant="title" id="templates-empty-heading">
        {t('page.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.emptyHelp')}
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
  template: Template
  summary?: AuthoringSummary
}) {
  const { t } = useTranslation('templates')
  return (
    // `min-h-16` and `py-2`, not the list row's usual `min-h-11`/`py-3`: every row carries the
    // delete, which keeps the 44px floor, so the row is 44 plus its own padding (THEME-23).
    <li className="hover:bg-row-bg-hover active:bg-row-bg-active relative flex min-h-16 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2 sm:px-6 lg:px-8">
      <Link
        to="/templates/$templateId"
        params={{ templateId: template.id }}
        className={typographyStyles({
          variant: 'label',
          className: 'min-w-0 truncate after:absolute after:inset-0',
        })}
      >
        {template.name}
      </Link>
      {template.description && (
        <Typography variant="meta" as="span" className="min-w-0 flex-1 truncate">
          {template.description}
        </Typography>
      )}
      <div className="basis-full">
        <Typography variant="body" className="text-content-secondary">
          {t('screen.savedAvailable')}
        </Typography>
        <EditingStatus summary={summary} />
      </div>
      <div className="relative ml-auto flex shrink-0 items-center gap-2">
        <Badge tone="neutral">{t('postCount', { count: template.postCount })}</Badge>
        <DeleteTemplateButton ownerId={ownerId} template={template} />
      </div>
    </li>
  )
}

function EditingStatus({ summary }: { summary?: AuthoringSummary }) {
  const { t } = useTranslation(['templates', 'authoring'])
  if (!summary) return null
  const named = {
    kind: t('kinds.post-template', { ns: 'authoring' }),
    name: summary.displayName || t('page.new', { ns: 'templates' }),
  }
  return (
    <div className="mt-1 space-y-1" role="status">
      {summary.lastPublication?.outcome && (
        <Typography variant="body">
          {t('screen.lastPublication', {
            ns: 'templates',
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
          {t('screen.activeAI', { ns: 'templates', name: named.name })}
        </Typography>
      )}
      {summary.publicationPending && (
        <Typography variant="body">
          {t('screen.publicationPending', { ns: 'templates', name: named.name })}
        </Typography>
      )}
      {summary.targetConflict && (
        <Typography variant="body">
          {t('screen.conflict', { ns: 'templates', name: named.name })}
        </Typography>
      )}
    </div>
  )
}
