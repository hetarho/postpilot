import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { type BillingEvent, useMyRefunds, useRequestRefund } from '@/entities/subscription'
import { formatDateTime, formatNumber } from '@/shared/lib'
import {
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  LoadingState,
  Notice,
  Textarea,
  Typography,
  cardStyles,
} from '@/shared/ui'

export function RefundRequestSection({ charges }: { charges: BillingEvent[] }) {
  const { t } = useTranslation('billing')
  const reasonId = useId()
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
        className={cardStyles({ className: 'grid gap-3' })}
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
        <div className="grid gap-1">
          <FieldLabel htmlFor={reasonId}>{t('refund.reason')}</FieldLabel>
          <Textarea
            id={reasonId}
            inputMode="text"
            autoComplete="off"
            autoCapitalize="sentences"
            autoCorrect="on"
            enterKeyHint="done"
            className="min-h-24"
            maxLength={500}
            required
            value={reason}
            onChange={(event) => setReason(event.target.value)}
          />
        </div>
        {mutation.isError && <FieldMessage>{t('refund.requestFailed')}</FieldMessage>}
        <Button
          type="submit"
          variant="secondary"
          disabled={mutation.isPending || candidates.length === 0}
        >
          {t('refund.request')}
        </Button>
      </form>
      {isPending && <LoadingState variant="inline" label={t('refund.loading')} />}
      {refunds.length > 0 && (
        <ul className="divide-divider divide-y">
          {refunds.map((refund) => (
            <li key={refund.id} className="py-4">
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
