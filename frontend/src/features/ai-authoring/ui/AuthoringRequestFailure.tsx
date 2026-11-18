import { useTranslation } from 'react-i18next'
import { AUTHORING_REFERENCE_REFUSALS } from '@/entities/ai-authoring'
import { useSession } from '@/entities/session'
import type { AppFailure } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { Button, Notice, TechnicalDetail, Typography } from '@/shared/ui'

/** Guide: an AI request that did not finish, said in three plain parts — what happened, from
 *  the failure's own reason and params (the model it could not use and why, or the reason the
 *  product already words); what is still here (the draft, or the message in the field); and
 *  what to do next, with the two ways to do it: send the same request again, or pick another
 *  model and send it with that one. Both go through the usual estimate before anything runs.
 *  The provider's own words stay behind the technical-detail disclosure, for a master account
 *  only: an owner reads the product's copy, never provider prose (QUOTA-66). */
export function AuthoringRequestFailure({
  failure,
  modelLabel,
  kept,
  onRetry,
  onOtherModel,
  className,
}: {
  failure: AppFailure
  /** The write model's label, for a MODEL_UNAVAILABLE that does not name its own. */
  modelLabel?: string
  kept?: 'draft' | 'message'
  onRetry: () => void
  /** Absent where the host offers no model choice. */
  onOtherModel?: () => void
  className?: string
}) {
  const { t } = useTranslation(['authoring', 'common'])
  const { user } = useSession()
  const modelFailure = failure.reason === 'MODEL_UNAVAILABLE'
  const model = failure.params.model || modelLabel
  const cause = failure.params.cause
  const refusal = AUTHORING_REFERENCE_REFUSALS.find((code) => code === failure.params.refusal)
  const happened = modelFailure
    ? model
      ? t('failure.modelUnavailableNamed', { model })
      : t('failure.modelUnavailable')
    : failure.reason === 'UNKNOWN_FAILURE'
      ? t('failed')
      : failure.reason === 'AUTHORING_REFERENCE_UNAVAILABLE' && refusal
        ? t(`references.refusal.${refusal}`)
        : formatAppFailure(failure)
  return (
    <Notice tone="danger" role="alert" className={className}>
      <div className="flex min-w-0 flex-col gap-1 break-words">
        <Typography variant="label" as="p">
          {happened}
        </Typography>
        {modelFailure && (cause === 'provider_policy' || cause === 'gone') && (
          <Typography variant="body" as="p">
            {t(`failure.${cause}`)}
          </Typography>
        )}
        {kept && (
          <Typography variant="body" as="p">
            {t(kept === 'draft' ? 'failure.keptDraft' : 'failure.keptMessage')}
          </Typography>
        )}
        <Typography variant="body" as="p">
          {t(modelFailure ? 'failure.nextModel' : 'failure.next')}
        </Typography>
        {user?.plan === 'master' && (
          <TechnicalDetail
            label={t('failure.technicalDetail', { ns: 'common' })}
            detail={failure.technicalDetail}
          />
        )}
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button variant={modelFailure && onOtherModel ? 'secondary' : 'cta'} onClick={onRetry}>
            {t('failure.retry')}
          </Button>
          {onOtherModel && (
            <Button variant={modelFailure ? 'cta' : 'secondary'} onClick={onOtherModel}>
              {t('failure.otherModel')}
            </Button>
          )}
        </div>
      </div>
    </Notice>
  )
}
