import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
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

/** The account's 템플릿 (plan 11): a list, and the one action that adds to it. Composition only —
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
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="templates-heading" className="mt-8">
              <Typography variant="title" id="templates-heading">
                {t('page.saved', { ns: 'templates' })}
              </Typography>
              {/* Rows are full-bleed against the page gutter, so the list cancels it (§4.2). */}
              <ul className="divide-divider -mx-4 mt-3 divide-y sm:-mx-6 lg:-mx-8">
                {templates.map((template) => (
                  <TemplateRow key={template.id} ownerId={ownerId} template={template} />
                ))}
              </ul>
            </section>
          )}

          {/* One instance at every width, and a PHONE dock: the reach argument is the only thing
              holding this bar up, and it evaporates with the thumb (§4.3). */}
          <ActionBar
            dock="phone"
            ariaLabel={t('page.newDockAria', { ns: 'templates' })}
            className="mt-auto"
          >
            <Link
              to="/templates/new"
              className={buttonStyles({ variant: 'cta', className: 'w-full' })}
            >
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
 *  editor teach its own vocabulary (change 30). It creates nothing: there are no shipped presets. */
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
 *  without navigating — a row is one target, not a row with a button inside it (§4.1). */
function TemplateRow({ ownerId, template }: { ownerId: string; template: Template }) {
  const { t } = useTranslation('templates')
  return (
    // `min-h-16` and `py-2`, not the list row's usual `min-h-11`/`py-3`: every row carries the
    // delete, which keeps the 44px floor, so the row is 44 plus its own padding (§4.2).
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
      <div className="relative ml-auto flex shrink-0 items-center gap-2">
        <Badge tone="neutral">{t('postCount', { count: template.postCount })}</Badge>
        <DeleteTemplateButton ownerId={ownerId} template={template} />
      </div>
    </li>
  )
}
