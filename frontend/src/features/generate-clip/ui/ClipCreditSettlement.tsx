import { useTranslation } from 'react-i18next'
import type { ClipAccounting } from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { Typography } from '@/shared/ui'

export function ClipCreditSettlement({
  job,
  accounting,
}: {
  job?: GenerationJob
  accounting?: ClipAccounting
}) {
  const { t } = useTranslation('clips')
  if (!job || job.kind !== 'generate_clip') return null
  const a = accounting?.jobId === job.id ? accounting : undefined
  const settled = !!a?.settled
  const fields = [
    ['approved', a?.approvedMaxCredits],
    ['reserved', a?.reservedCredits],
    ['charged', settled ? a?.finalChargeCredits : undefined],
    ['refunded', settled ? a?.refundCredits : undefined],
  ] as const
  const breakdown = settled
    ? ([
        ['confirmed', a?.confirmedChargeCredits],
        ['cancellationFee', a?.cancellationFeeCredits],
        ...(a?.status === 'exempt'
          ? ([
              ['shadowConfirmed', a.shadowConfirmedChargeCredits],
              ['shadowCancellationFee', a.shadowCancellationFeeCredits],
              ['shadowTotal', a.shadowChargeCredits],
            ] as const)
          : []),
      ] as const)
    : []
  return (
    <section aria-label={t('credits.title')} className="mt-6 space-y-3">
      <Typography variant="title">{t('credits.title')}</Typography>
      <Typography variant="body" role="status">
        {t(
          !a || a.status === 'unavailable'
            ? 'credits.unavailable'
            : !settled && isTerminal(job)
              ? 'credits.settling'
              : a.status === 'exempt'
                ? 'credits.exempt'
                : settled
                  ? 'credits.settled'
                  : a.status === 'reserved'
                    ? 'credits.held'
                    : 'credits.preparing',
        )}
      </Typography>
      <dl className="grid grid-cols-2 gap-x-4 gap-y-2">
        {[...fields, ...breakdown.filter(([, amount]) => amount !== undefined)].map(
          ([label, amount]) => (
            <div key={label} className="min-w-0 space-y-1">
              <dt>
                <Typography variant="meta">
                  {t(
                    label === 'confirmed' ||
                      label === 'cancellationFee' ||
                      label === 'shadowConfirmed' ||
                      label === 'shadowCancellationFee' ||
                      label === 'shadowTotal'
                      ? `cancellation.${label}`
                      : `credits.${label}`,
                  )}
                </Typography>
              </dt>
              <dd>
                <Typography variant="body">
                  {amount === undefined
                    ? t('credits.pendingAmount')
                    : t('credits.amount', { amount })}
                </Typography>
              </dd>
            </div>
          ),
        )}
      </dl>
      {settled && job.status === 'failed' && a?.finalChargeCredits === 0 && (
        <Typography variant="body">{t('credits.noCharge')}</Typography>
      )}
    </section>
  )
}
