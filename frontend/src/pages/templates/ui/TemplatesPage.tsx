import { useTranslation } from 'react-i18next'
import { useRef, type Ref } from 'react'
import { Link } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useAuthoringSummaries, type AuthoringSummary } from '@/entities/ai-authoring'
import { useTemplates, type Template } from '@/entities/template'
import { DeleteTemplateButton } from '@/features/delete-template'
import { ChannelActions } from '@/widgets/channel-actions'
import { LayoutList } from 'lucide-react'
import {
  Badge,
  Button,
  Notice,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
  useListRemovalFocus,
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
  const page = useRef<HTMLElement>(null)
  const deleteButtons = useRef(new Map<string, HTMLButtonElement>())
  const requestRemoval = useListRemovalFocus({
    keys: templates.map((template) => template.id),
    getTarget: (id) => deleteButtons.current.get(id),
    getFallback: () => page.current?.querySelector<HTMLAnchorElement>('a[href="/templates/new"]'),
  })

  return (
    <main
      ref={page}
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}
    >
      {/* Guide: the hub's title and the 템플릿 chip say where this is; the page leads with what a
          template does. */}
      <h1 className="sr-only">{t('title', { ns: 'templates' })}</h1>
      <Typography variant="display" as="p" aria-hidden="true">
        {t('page.lead', { ns: 'templates' })}
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
            <section aria-labelledby="templates-heading" className="mt-6">
              <Typography variant="title" id="templates-heading">
                {t('page.saved', { ns: 'templates' })}
              </Typography>
              {/* Guide: each template is a card; the card opens its own focused screen. */}
              <ul className="mt-3 grid gap-3 lg:grid-cols-2">
                {templates.map((template, index) => (
                  <TemplateRow
                    key={template.id}
                    ownerId={ownerId}
                    template={template}
                    deleteButtonRef={(button) => {
                      if (button) deleteButtons.current.set(template.id, button)
                      else deleteButtons.current.delete(template.id)
                    }}
                    onDeleted={() => requestRemoval(template.id, index)}
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

          {/* The section's creation button is the page's one dock (THEME-67). */}
          <ChannelActions section="templates" />
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
  deleteButtonRef,
  onDeleted,
}: {
  ownerId: string
  template: Template
  summary?: AuthoringSummary
  deleteButtonRef: Ref<HTMLButtonElement>
  onDeleted: () => void
}) {
  const { t } = useTranslation('templates')
  return (
    // Guide: a raised card — a glyph tile, the name in a heading's size and the description under
    // it. The link stretches over the whole card through its `::after`; the delete paints above.
    <li
      className={cardStyles({
        interactive: true,
        padding: 'sm',
        className: 'relative flex min-h-16 flex-wrap items-start gap-x-3 gap-y-2 sm:p-5',
      })}
    >
      <span
        aria-hidden="true"
        className="bg-badge-accent-bg text-badge-accent-fg flex size-11 shrink-0 items-center justify-center rounded-lg"
      >
        <LayoutList className="size-5" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <Link
          to="/templates/$templateId"
          params={{ templateId: template.id }}
          className={typographyStyles({
            variant: 'fieldTitle',
            className: 'text-content-primary min-w-0 truncate after:absolute after:inset-0',
          })}
        >
          {template.name}
        </Link>
        {template.description && (
          <Typography variant="body" as="span" className="text-content-secondary line-clamp-2">
            {template.description}
          </Typography>
        )}
      </span>
      <div className="basis-full">
        <Typography variant="body" className="text-content-secondary">
          {t('screen.savedAvailable')}
        </Typography>
        <EditingStatus summary={summary} />
      </div>
      <div className="relative ml-auto flex shrink-0 items-center gap-2">
        <Badge tone="neutral">{t('postCount', { count: template.postCount })}</Badge>
        <DeleteTemplateButton
          ownerId={ownerId}
          template={template}
          buttonRef={deleteButtonRef}
          onDeleted={onDeleted}
        />
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
