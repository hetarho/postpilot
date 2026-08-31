import { useTranslation } from 'react-i18next'
import { useSession } from '@/entities/session'
import { PURPOSE_LIMITS, usePurposes, type Purpose } from '@/entities/purpose'
import { CreatePurposeForm } from '@/features/create-purpose'
import { DeletePurposeButton } from '@/features/delete-purpose'
import { EditablePurposeField, useUpdatePurpose } from '@/features/edit-purpose'
import { Badge, Button, Notice } from '@/shared/ui'

/** The account's 용도 briefs (plan 11). Composition only — every action is its own feature.
 *
 *  Nothing on this screen calls a model or enqueues a job: a purpose is authored text, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5]). */
export function PurposesPage() {
  const { t } = useTranslation(['purposes', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { purposes, isPending, isError, isFetching, refetch } = usePurposes(ownerId)

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">{t('title', { ns: 'purposes' })}</h1>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        {t('page.description', { ns: 'purposes' })}
      </p>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed', { ns: 'purposes' })}</span>
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
        <p role="status" className="text-content-tertiary mt-8 text-sm">
          {t('state.loading', { ns: 'common' })}
        </p>
      )}

      {!isError && !isPending && (
        <>
          {purposes.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="purposes-heading" className="mt-8">
              <h2 id="purposes-heading" className="text-lg font-semibold tracking-tight">
                {t('page.saved', { ns: 'purposes' })}
              </h2>
              <ul className="divide-divider mt-3 divide-y">
                {purposes.map((purpose) => (
                  <PurposeRow key={purpose.id} ownerId={ownerId} purpose={purpose} />
                ))}
              </ul>
            </section>
          )}

          <section aria-labelledby="create-purpose-heading" className="mt-10">
            <h2 id="create-purpose-heading" className="text-lg font-semibold tracking-tight">
              {t('page.new', { ns: 'purposes' })}
            </h2>
            <CreatePurposeForm ownerId={ownerId} className="mt-3" />
          </section>
        </>
      )}
    </main>
  )
}

/** The worked example is copy, not a row: nothing here creates a purpose the user did not
 *  author (plan 11 — no seeded presets, no guessing). */
function EmptyState() {
  const { t } = useTranslation('purposes')
  return (
    <section aria-labelledby="purposes-empty-heading" className="mt-8">
      <h2 id="purposes-empty-heading" className="text-lg font-semibold tracking-tight">
        {t('page.empty')}
      </h2>
      <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
        {t('page.emptyHelp')}
      </p>
      <dl className="text-content-secondary mt-4 space-y-2 text-sm leading-relaxed">
        <div>
          <dt className="text-content-tertiary text-xs font-medium">{t('page.name')}</dt>
          <dd className="text-content-primary">{t('page.exampleName')}</dd>
        </div>
        <div>
          <dt className="text-content-tertiary text-xs font-medium">{t('create.description')}</dt>
          <dd>{t('page.exampleDescription')}</dd>
        </div>
        <div>
          <dt className="text-content-tertiary text-xs font-medium">{t('create.instructions')}</dt>
          <dd className="whitespace-pre-wrap">{t('page.exampleInstructions')}</dd>
        </div>
      </dl>
    </section>
  )
}

/** One saved purpose: three read-first fields, the assignment count, and the delete.
 *
 *  Each field saves on its own so two of them edited from two places cannot overwrite each
 *  other (spec/policy/purposes.md). One mutation hook serves all three — they never run at
 *  the same time, and sharing it keeps one refusal message under one field. */
function PurposeRow({ ownerId, purpose }: { ownerId: string; purpose: Purpose }) {
  const { t } = useTranslation('purposes')
  const update = useUpdatePurpose(ownerId, purpose.id)
  return (
    <li className="py-4">
      <EditablePurposeField
        label={t('page.name')}
        value={purpose.name}
        limit={PURPOSE_LIMITS.name}
        placeholder={t('create.namePlaceholder')}
        save={update.saveName}
        errorMessage={update.errorMessage}
        pending={update.isPending}
      />
      <EditablePurposeField
        label={t('create.description')}
        value={purpose.description}
        limit={PURPOSE_LIMITS.description}
        multiline
        optional
        placeholder={t('create.descriptionPlaceholder')}
        emptyText={t('emptyDescription')}
        save={update.saveDescription}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-4"
      />
      <EditablePurposeField
        label={t('create.instructions')}
        value={purpose.instructions}
        limit={PURPOSE_LIMITS.instructions}
        multiline
        placeholder={t('create.instructionsPlaceholder').split('\n')[0]}
        save={update.saveInstructions}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-4"
      />
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Badge tone="neutral">{t('postCount', { count: purpose.postCount })}</Badge>
        <DeletePurposeButton ownerId={ownerId} purpose={purpose} />
      </div>
    </li>
  )
}
