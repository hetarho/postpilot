import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  type BillingRefundRequest,
  useReconcileRefund,
  useRefundReviews,
  useReviewRefund,
} from '@/entities/subscription'
import { formatDateTime, formatNumber } from '@/shared/lib'
import { AppFailureMessage, Button, Notice, Typography } from '@/shared/ui'

export function RefundReviewSection() {
  const { t } = useTranslation('billing')
  const { refunds, isPending, isError } = useRefundReviews()
  return (
    <section className="mt-10 grid gap-3">
      <Typography variant="title" as="h2">
        {t('refundReview.heading')}
      </Typography>
      {isPending && <Typography variant="body">{t('refundReview.loading')}</Typography>}
      {isError && (
        <Notice tone="danger" role="alert">
          {t('refundReview.loadFailed')}
        </Notice>
      )}
      {!isPending && !isError && refunds.length === 0 && (
        <Typography variant="body">{t('refundReview.empty')}</Typography>
      )}
      <ul className="grid gap-3">
        {refunds.map((item) => (
          <RefundReviewRow key={item.id} item={item} />
        ))}
      </ul>
    </section>
  )
}

function RefundReviewRow({ item }: { item: BillingRefundRequest }) {
  const { t } = useTranslation('billing')
  const statusLabels: Record<string, string> = {
    requested: t('refund.status.requested'),
    rejected: t('refund.status.rejected'),
    processing: t('refund.status.processing'),
    completed: t('refund.status.completed'),
    failed: t('refund.status.failed'),
  }
  const review = useReviewRefund()
  const reconcile = useReconcileRefund()
  const failure = review.failure ?? reconcile.failure
  const [amount, setAmount] = useState(String(item.payment?.chargedKrw ?? 0n))
  const validAmount =
    /^\d+$/.test(amount) &&
    BigInt(amount) > 0n &&
    BigInt(amount) <= (item.payment?.chargedKrw ?? 0n) - item.priorRefundedKrw
  const evidence = item.evidence
  return (
    <li className="border-divider grid gap-2 rounded-xl border p-4">
      <Typography variant="label">
        {item.userId} · {formatDateTime(item.requestedAt)} ·{' '}
        {statusLabels[item.status] ?? item.status}
      </Typography>
      <Typography variant="body">
        {t('refundReview.funded', {
          amount: formatNumber(item.payment?.chargedKrw ?? 0n),
          refunded: formatNumber(item.priorRefundedKrw),
          order: item.payment?.orderId ?? '',
        })}
      </Typography>
      <Typography variant="body">{item.reason}</Typography>
      <Typography variant="meta">
        {t('refundReview.evidence', {
          jobs: evidence?.paidModelJobs ?? 0,
          credits: evidence?.creditsUsed ?? 0,
          reserved: evidence?.creditsReserved ?? 0,
          exports: evidence?.serverExportsUsed ?? 0,
          exportReserved: evidence?.serverExportsReserved ?? 0,
        })}
      </Typography>
      <Typography variant="meta">{t('refundReview.disposition')}</Typography>
      {item.status === 'requested' && (
        <div className="flex flex-wrap items-end gap-2">
          <label className="grid gap-1">
            <Typography variant="label">{t('refundReview.amount')}</Typography>
            <input
              type="number"
              min="1"
              max={String(item.payment?.chargedKrw ?? 0n)}
              step="1"
              value={amount}
              onChange={(event) => setAmount(event.target.value)}
              className="border-divider bg-surface-raised w-40 rounded-md border p-2"
            />
          </label>
          <Button
            variant="secondary"
            disabled={review.isPending || !validAmount}
            onClick={() =>
              void review.reviewRefund(item.id, 'approve', BigInt(amount)).catch(() => undefined)
            }
          >
            {t('refundReview.approve')}
          </Button>
          <Button
            variant="secondary"
            disabled={review.isPending}
            onClick={() => void review.reviewRefund(item.id, 'reject', 0n).catch(() => undefined)}
          >
            {t('refundReview.reject')}
          </Button>
        </div>
      )}
      {item.status === 'processing' && (
        <Button
          variant="secondary"
          disabled={reconcile.isPending}
          onClick={() => void reconcile.reconcileRefund(item.id).catch(() => undefined)}
        >
          {t('refundReview.reconcile')}
        </Button>
      )}
      {failure && (
        <Notice tone="danger" role="alert">
          {/* A refusal the server names (the provider's, REFUND_FAILED) says why; an untyped
              failure keeps the screen's own line. */}
          {failure.reason === 'UNKNOWN_FAILURE' ? (
            t('refundReview.actionFailed')
          ) : (
            <AppFailureMessage failure={failure} />
          )}
        </Notice>
      )}
      {item.status === 'completed' && (
        <Typography variant="body">
          {t('refund.confirmedAmount', { amount: formatNumber(item.confirmedAmountKrw) })}
        </Typography>
      )}
    </li>
  )
}
