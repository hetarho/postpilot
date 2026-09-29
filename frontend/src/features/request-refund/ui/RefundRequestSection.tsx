import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { type BillingEvent, useMyRefunds, useRequestRefund } from '@/entities/subscription'
import { formatDateTime, formatNumber } from '@/shared/lib'
import { Button, FieldMessage, Listbox, Notice, Typography } from '@/shared/ui'

export function RefundRequestSection({ charges }: { charges: BillingEvent[] }) {
  const { t } = useTranslation('billing')
  const { refunds, isPending, isError } = useMyRefunds()
  const mutation = useRequestRefund()
  const [orderId, setOrderId] = useState('')
  const [reason, setReason] = useState('')
  const [sent, setSent] = useState(false)
  const statusLabels: Record<string, string> = {
    requested: t('refund.status.requested'),
    rejected: t('refund.status.rejected'),
    processing: t('refund.status.processing'),
    completed: t('refund.status.completed'),
    failed: t('refund.status.failed'),
  }
  const candidates = charges.filter(
    (event) => event.kind === 'charge' && event.orderId && event.krw > 0n,
  )

  const submit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (!orderId || !reason.trim()) return
    try {
      await mutation.requestRefund(orderId, reason.trim())
      setReason('')
      setOrderId('')
      setSent(true)
    } catch {
      /* Error remains next to the form. */
    }
  }

  return (
    <section className="grid gap-3">
      <Typography variant="title" as="h2">
        {t('refund.heading')}
      </Typography>
      <Typography variant="body" className="text-content-secondary">
        {t('refund.policy')}
      </Typography>
      {sent && (
        <Notice tone="success" role="status">
          {t('refund.requested')}
        </Notice>
      )}
      {isError && (
        <Notice tone="danger" role="alert">
          {t('refund.loadFailed')}
        </Notice>
      )}
      <form
        onSubmit={(event) => void submit(event)}
        className="border-divider grid gap-3 rounded-xl border p-4"
      >
        <div className="grid gap-1">
          <Typography variant="label">{t('refund.payment')}</Typography>
          <Listbox
            aria-label={t('refund.payment')}
            value={orderId}
            onChange={setOrderId}
            placeholder={t('refund.select')}
            options={candidates.map((charge) => ({
              value: charge.orderId,
              label: `${formatDateTime(charge.createdAt)} · ${formatNumber(charge.krw)}원 · ${charge.orderId}`,
            }))}
          />
        </div>
        <label className="grid gap-1">
          <Typography variant="label">{t('refund.reason')}</Typography>
          <textarea
            className="border-divider bg-surface-raised min-h-24 rounded-md border p-2"
            maxLength={500}
            required
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </label>
        {mutation.isError && <FieldMessage>{t('refund.requestFailed')}</FieldMessage>}
        <Button
          type="submit"
          variant="secondary"
          disabled={mutation.isPending || candidates.length === 0}
        >
          {t('refund.request')}
        </Button>
      </form>
      {isPending && <Typography variant="body">{t('refund.loading')}</Typography>}
      {refunds.length > 0 && (
        <ul className="grid gap-2">
          {refunds.map((refund) => (
            <li key={refund.id} className="border-divider rounded-md border p-3">
              <Typography variant="label">
                {formatDateTime(refund.requestedAt)} · {refund.payment?.orderId}
              </Typography>
              <Typography variant="body">
                {statusLabels[refund.status] ?? refund.status} · {refund.reason}
              </Typography>
              {refund.status === 'completed' && (
                <Typography variant="body">
                  {t('refund.confirmedAmount', { amount: formatNumber(refund.confirmedAmountKrw) })}
                </Typography>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
