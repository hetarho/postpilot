import { useTranslation } from 'react-i18next'
import type { ClipAccounting } from '@/entities/clip-project'
import type { GenerationJob } from '@/entities/generation-job'
import { AppFailureMessage, Button, Typography } from '@/shared/ui'
import type { useCancelClip } from '../model/useCancelClip'

export function CancelClipAction({
  action,
  job,
  accounting,
}: {
  action: ReturnType<typeof useCancelClip>
  job?: GenerationJob
  accounting?: ClipAccounting
}) {
  const { t } = useTranslation('clips')
  const a = accounting?.jobId === job?.id ? accounting : undefined
  const legacy = job?.kind === 'generate_clip' && job.cancellationPolicyVersion !== 1
  return (
    <div className="space-y-3">
      <Typography variant="body">
        {t(
          legacy
            ? 'cancellation.legacy'
            : job?.kind === 'render_clip'
              ? 'cancellation.free'
              : a?.status === 'exempt'
                ? 'cancellation.exempt'
                : a?.status === 'not_reserved'
                  ? 'cancellation.beforeReservation'
                  : 'cancellation.rule',
        )}
      </Typography>
      {a?.status === 'reserved' && a.reservedCredits !== undefined && (
        <Typography variant="meta">
          {t('cancellation.reservation', { amount: a.reservedCredits })}
        </Typography>
      )}
      {action.failure && (
        <div role="alert">
          <AppFailureMessage failure={action.failure} />
        </div>
      )}
      {action.uncertain && (
        <>
          <Typography variant="body">{t('cancellation.uncertain')}</Typography>
          <Button
            variant="secondary"
            pending={action.pending}
            onClick={() => void action.checkAgain()}
          >
            {t('project.retry')}
          </Button>
        </>
      )}
      <Button
        variant="secondary"
        className="w-full sm:w-auto"
        pending={action.pending}
        disabled={!job?.canCancel || legacy || action.cancelling || action.uncertain}
        onClick={() => void action.cancel()}
      >
        {t(action.cancelling ? 'cancellation.cancelling' : 'cancellation.cancel')}
      </Button>
    </div>
  )
}
