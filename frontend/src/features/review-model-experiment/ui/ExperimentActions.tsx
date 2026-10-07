import { useTranslation } from 'react-i18next'
import type { ModelExperiment } from '@/entities/model-experiment'
import { formatDateTime } from '@/shared/lib'
import { AppFailureMessage, Notice, Typography, buttonStyles } from '@/shared/ui'

/** Retained comparison receipts are facts, rather than invitations to replay a legacy action. */
export function ExperimentActions({
  experiment,
  activeCandidateId,
}: {
  experiment: ModelExperiment
  activeCandidateId: string
}) {
  void activeCandidateId
  const { t } = useTranslation('models')
  return (
    <div className="grid gap-3">
      <Notice tone="info" role="status">
        {t('legacyReview.readOnly')}
      </Notice>
      {experiment.appliedAt && (
        <Typography variant="label" as="p">
          {t('legacyReview.applied', { when: formatDateTime(experiment.appliedAt) })}
        </Typography>
      )}
      {experiment.adoptedAt && (
        <Typography variant="label" as="p">
          {t('legacyReview.adopted', { when: formatDateTime(experiment.adoptedAt) })}
        </Typography>
      )}
      {experiment.applyFailure && (
        <Notice tone="warning">
          <AppFailureMessage failure={experiment.applyFailure} />
        </Notice>
      )}
      {experiment.adoptionFailure && (
        <Notice tone="warning">
          <AppFailureMessage failure={experiment.adoptionFailure} />
        </Notice>
      )}
      <a
        href="/tests"
        className={buttonStyles({ variant: 'secondary', className: 'justify-self-start' })}
      >
        {t('legacyReview.openTests')}
      </a>
    </div>
  )
}
