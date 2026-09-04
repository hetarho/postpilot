import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { TEMPLATE_LIMITS, useTemplates, type Template } from '@/entities/template'
import { CreateTemplateForm } from '@/features/create-template'
import { DeleteTemplateButton } from '@/features/delete-template'
import {
  EditableTemplateField,
  EditableTemplateBody,
  useUpdateTemplate,
} from '@/features/edit-template'
import { Badge, Button, Notice, Typography, typographyStyles, pageStyles } from '@/shared/ui'

/** The account's 템플릿 briefs (plan 11). Composition only — every action is its own feature.
 *
 *  Nothing on this screen calls a model or enqueues a job: a template is authored text, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5]). */
export function TemplatesPage() {
  const { t } = useTranslation(['templates', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)

  return (
    <main className={pageStyles({ width: 'wide' })}>
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
              <ul className="divide-divider mt-3 divide-y">
                {templates.map((template) => (
                  <TemplateRow key={template.id} ownerId={ownerId} template={template} />
                ))}
              </ul>
            </section>
          )}

          <section aria-labelledby="create-template-heading" className="mt-10">
            <Typography variant="title" id="create-template-heading">
              {t('page.new', { ns: 'templates' })}
            </Typography>
            <CreateTemplateForm ownerId={ownerId} className="mt-3" />
          </section>
        </>
      )}
    </main>
  )
}

/** The worked example is copy, not a row: nothing here creates a template the user did not
 *  author (plan 11 — no seeded presets, no guessing). */
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
      <dl
        className={typographyStyles({
          variant: 'body',
          className: 'text-content-secondary mt-4 space-y-2',
        })}
      >
        <div>
          <dt className={typographyStyles({ variant: 'label' })}>{t('page.name')}</dt>
          <dd className="text-content-primary">{t('page.exampleName')}</dd>
        </div>
        <div>
          <dt className={typographyStyles({ variant: 'label' })}>{t('create.description')}</dt>
          <dd>{t('page.exampleDescription')}</dd>
        </div>
        <div>
          <dt className={typographyStyles({ variant: 'label' })}>{t('create.body')}</dt>
          <dd className="whitespace-pre-wrap">{t('page.exampleBody')}</dd>
        </div>
      </dl>
    </section>
  )
}

/** One saved template: three read-first fields, the assignment count, and the delete.
 *
 *  Each field saves on its own so two of them edited from two places cannot overwrite each
 *  other (spec/policy/templates.md). One mutation hook serves all three — they never run at
 *  the same time, and sharing it keeps one refusal message under one field. */
function TemplateRow({ ownerId, template }: { ownerId: string; template: Template }) {
  const { t } = useTranslation('templates')
  const update = useUpdateTemplate(ownerId, template.id)
  return (
    <li className="py-4">
      <EditableTemplateField
        label={t('page.name')}
        value={template.name}
        limit={TEMPLATE_LIMITS.name}
        placeholder={t('create.namePlaceholder')}
        save={update.saveName}
        errorMessage={update.errorMessage}
        pending={update.isPending}
      />
      <EditableTemplateField
        label={t('create.description')}
        value={template.description}
        limit={TEMPLATE_LIMITS.description}
        multiline
        optional
        placeholder={t('create.descriptionPlaceholder')}
        emptyText={t('emptyDescription')}
        save={update.saveDescription}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-4"
      />
      <EditableTemplateBody
        value={template.body}
        save={update.saveBody}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-4"
      />
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Badge tone="neutral">{t('postCount', { count: template.postCount })}</Badge>
        <DeleteTemplateButton ownerId={ownerId} template={template} />
      </div>
    </li>
  )
}
