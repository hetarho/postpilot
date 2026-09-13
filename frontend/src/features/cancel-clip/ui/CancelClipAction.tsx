import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipAccounting } from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { AppFailureMessage, Button, Dialog, Typography } from '@/shared/ui'
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
  const [confirmationJobId, setConfirmationJobId] = useState<string>()
  const a = accounting?.jobId === job?.id ? accounting : undefined
  const legacy = job?.kind === 'generate_clip' && job.cancellationPolicyVersion !== 1
  const available =
    !!job?.canCancel &&
    !legacy &&
    !isTerminal(job) &&
    !job?.cancelRequestedAt &&
    !action.cancelling &&
    !action.uncertain &&
    !action.pending
  const confirming = !!confirmationJobId && confirmationJobId === job?.id && available
  const rule = legacy
    ? 'cancellation.legacy'
    : job?.kind === 'render_clip'
      ? 'cancellation.free'
      : a?.status === 'exempt'
        ? 'cancellation.exempt'
        : a?.status === 'not_reserved'
          ? 'cancellation.beforeReservation'
          : 'cancellation.rule'
  return (
    <div className="space-y-3">
      <Typography variant="body">{t(rule)}</Typography>
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
        disabled={!available}
        onClick={() => setConfirmationJobId(job?.id)}
      >
        {t(action.cancelling ? 'cancellation.cancelling' : 'cancellation.cancel')}
      </Button>
      <Dialog
        open={confirming}
        title={t('cancellation.confirmTitle')}
        cancelLabel={t('cancellation.continueProduction')}
        confirmLabel={t('cancellation.confirmCancel')}
        onClose={() => setConfirmationJobId(undefined)}
        onConfirm={() => {
          if (!confirming) return
          setConfirmationJobId(undefined)
          void action.cancel()
        }}
      >
        <div className="space-y-3">
          <Typography variant="body">{t('cancellation.confirmHelp')}</Typography>
          <Typography variant="body">{t(rule)}</Typography>
          {a?.status === 'reserved' && a.reservedCredits !== undefined && (
            <Typography variant="body">
              {t('cancellation.reservation', { amount: a.reservedCredits })}
            </Typography>
          )}
        </div>
      </Dialog>
    </div>
  )
}
