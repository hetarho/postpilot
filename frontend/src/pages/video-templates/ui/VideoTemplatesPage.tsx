import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipTemplates, type ClipTemplate } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
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
          {templates.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="video-templates-heading" className="mt-8">
              <Typography variant="title" id="video-templates-heading">
                {t('directory.saved', { ns: 'clips' })}
              </Typography>
              {/* Rows are full-bleed against the page gutter, so the list cancels it (§4.2). */}
              <ul className="divide-divider -mx-4 mt-3 divide-y sm:-mx-6 lg:-mx-8">
                {templates.map((template) => (
                  <TemplateRow key={template.id} ownerId={ownerId} template={template} />
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
            <Link
              to="/video-templates/new"
              className={buttonStyles({ variant: 'cta', className: 'w-full sm:w-auto' })}
            >
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
 *  without navigating — a row is one target, not a row with a button inside it (§4.1). */
function TemplateRow({ ownerId, template }: { ownerId: string; template: ClipTemplate }) {
  const { t } = useTranslation('clips')
  return (
    // `min-h-16` and `py-2`, not the list row's usual `min-h-11`/`py-3`: every row carries the
    // delete, which keeps the 44px floor, so the row is 44 plus its own padding (§4.2).
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
      <Typography variant="meta" as="span" className="min-w-0 flex-1 truncate">
        {t('directory.fields', { count: template.informationFields.length })}
        {template.copyStyles.length > 0 &&
          ` · ${template.copyStyles.map((style) => t(`style.${style}`)).join(' · ')}`}
      </Typography>
      <div className="relative ml-auto flex shrink-0 items-center gap-2">
        <Badge tone="neutral">
          {t('directory.projectCount', { count: template.projectCount })}
        </Badge>
        <DeleteClipTemplateButton ownerId={ownerId} template={template} />
      </div>
    </li>
  )
}
