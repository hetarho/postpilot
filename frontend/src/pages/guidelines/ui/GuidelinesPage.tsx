import { useTranslation } from 'react-i18next'
import { useGuidelines, useUpdateGuidelineCall, type Guideline } from '@/entities/guideline'
import { useSession } from '@/entities/session'
import { CreateGuidelineForm } from '@/features/create-guideline'
import { DeleteGuidelineButton } from '@/features/delete-guideline'
import { EditableGuidelineScope, EditableGuidelineText } from '@/features/edit-guideline'
import { Button, Notice, Typography } from '@/shared/ui'

/** The account's 작문 지침 (plan 16). Composition only — every action is its own feature.
 *
 *  Nothing on this screen calls a model or enqueues a job: a guideline is authored text, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5]). The list is rendered in the
 *  server's order because that order IS the injection order the writer will see. */
export function GuidelinesPage() {
  const { t } = useTranslation(['guidelines', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { guidelines, isPending, isError, isFetching, refetch } = useGuidelines(ownerId)

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <Typography variant="display">{t('title', { ns: 'guidelines' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.description', { ns: 'guidelines' })}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed', { ns: 'guidelines' })}</span>
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
          {guidelines.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="guidelines-heading" className="mt-8">
              <Typography variant="title" id="guidelines-heading">
                {t('page.saved', { ns: 'guidelines' })}
              </Typography>
              <Typography variant="body" as="p" className="text-content-secondary mt-1">
                {t('page.order', { ns: 'guidelines' })}
              </Typography>
              <ul className="divide-divider mt-3 divide-y">
                {guidelines.map((guideline) => (
                  <GuidelineRow key={guideline.id} ownerId={ownerId} guideline={guideline} />
                ))}
              </ul>
            </section>
          )}

          <section aria-labelledby="create-guideline-heading" className="mt-10">
            <Typography variant="title" id="create-guideline-heading">
              {t('page.new', { ns: 'guidelines' })}
            </Typography>
            <CreateGuidelineForm ownerId={ownerId} className="mt-3" />
          </section>
        </>
      )}
    </main>
  )
}

/** The worked example is copy, not a row: nothing here creates a guideline the user did not
 *  author (plan 16 — no seeded library, no inference). */
function EmptyState() {
  const { t } = useTranslation('guidelines')
  return (
    <section aria-labelledby="guidelines-empty-heading" className="mt-8">
      <Typography variant="title" id="guidelines-empty-heading">
        {t('page.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.emptyHelp')}
      </Typography>
      <Typography variant="body" className="text-content-primary mt-3">
        {t('page.example')}
      </Typography>
    </section>
  )
}

/** One saved guideline: its text and its scope, each read-first and each saving on its own so the
 *  two edited from two places cannot overwrite each other. One mutation hook serves both — they
 *  never run at the same time, and sharing it keeps one refusal message under one field. */
function GuidelineRow({ ownerId, guideline }: { ownerId: string; guideline: Guideline }) {
  const update = useUpdateGuidelineCall(ownerId, guideline.id)
  return (
    <li className="py-4">
      <EditableGuidelineText
        value={guideline.text}
        save={update.saveText}
        errorMessage={update.errorMessage}
        pending={update.isPending}
      />
      <EditableGuidelineScope
        ownerId={ownerId}
        guideline={guideline}
        save={update.saveScope}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-3"
      />
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <DeleteGuidelineButton ownerId={ownerId} guidelineId={guideline.id} />
      </div>
    </li>
  )
}
