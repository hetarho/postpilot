import { useTranslation } from 'react-i18next'
import type { ClipAccounting } from '@/entities/clip-project'
import { isTerminal, type GenerationJob } from '@/entities/generation-job'
import { formatDateTime } from '@/shared/lib'
import { Typography } from '@/shared/ui'

/** What this generation cost, as ONE figure (CLIP-81, owner decision 2026-09-19): the credits it
 *  used once settled, and a pending word until then. The ceiling, the reservation, the refund and
 *  the cancellation split are the server's bookkeeping and stay out of the workspace.
 *
 *  An exempt (master) account is never debited; the server records what the run WOULD have cost
 *  as a reference amount, and that is the figure shown for it, marked as not debited — the one
 *  thing the old breakdown said that the owner could not read from it. */
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
  const exempt = a?.status === 'exempt'
  const reference = a?.shadowConfirmedChargeCredits ?? a?.shadowChargeCredits ?? 0
  const value =
    !a || a.status === 'unavailable'
      ? t('credits.pendingAmount')
      : settled
        ? t('credits.used', { amount: exempt ? reference : (a.finalChargeCredits ?? 0) })
        : isTerminal(job)
          ? t('credits.settling')
          : t('credits.pendingAmount')
  return (
    <section
      aria-label={t('credits.title')}
      className="mt-4 flex flex-wrap items-baseline gap-x-3 gap-y-1"
    >
      <Typography variant="meta" as="h2">
        {t('credits.title')}
      </Typography>
      <Typography variant="body" role="status">
        {value}
      </Typography>
      {settled && !exempt && a && (
        <div className="grid w-full gap-1">
          <Typography variant="meta" as="span">
            {t('credits.debit', { amount: a.confirmedChargeCredits ?? a.finalChargeCredits ?? 0 })}
          </Typography>
          {(a.refundCredits ?? 0) > 0 && (
            <Typography variant="meta" as="span">
              {t('credits.returned', { amount: a.refundCredits ?? 0 })}
            </Typography>
          )}
          {(a.compensationCredits ?? 0) > 0 && (
            <Typography variant="meta" as="span">
              {t('credits.compensation', {
                amount: a.compensationCredits ?? 0,
                at: a.compensationExpiresAt ? formatDateTime(a.compensationExpiresAt) : '—',
              })}
            </Typography>
          )}
          <Typography variant="meta" as="span">
            {t('credits.net', { amount: a.netDebitCredits ?? a.finalChargeCredits ?? 0 })}
          </Typography>
        </div>
      )}
    </section>
  )
}
