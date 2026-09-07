import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  usePurchaseCredits,
  useQuotePurchase,
  useRefundPurchase,
  type Purchase,
} from '@/entities/subscription'
import { formatDateTime, formatNumber } from '@/shared/lib'
import { Button, Dialog, FieldMessage, Notice, Stepper, Typography } from '@/shared/ui'

export function CreditPurchaseSection({
  hasPaymentMethod,
  purchases,
}: {
  hasPaymentMethod: boolean
  purchases: Purchase[]
}) {
  const { t } = useTranslation('billing')
  const [dollars, setDollars] = useState(5)
  const [confirmingPurchase, setConfirmingPurchase] = useState(false)
  const [refunding, setRefunding] = useState<Purchase | undefined>()
  const [success, setSuccess] = useState<{ kind: 'purchase' | 'refund'; credits: number }>()
  const { quote, isPending: quotePending, isError: quoteError } = useQuotePurchase(dollars * 100)
  const purchaseMutation = usePurchaseCredits()
  const refundMutation = useRefundPurchase()

  const confirmPurchase = async () => {
    try {
      const purchased = await purchaseMutation.purchaseCredits(dollars * 100)
      if (!purchased) return
      setConfirmingPurchase(false)
      setSuccess({ kind: 'purchase', credits: purchased.credits })
    } catch {
      // The catalog-backed failure remains in the confirmation dialog.
    }
  }

  const confirmRefund = async () => {
    if (!refunding) return
    try {
      const refunded = await refundMutation.refundPurchase(refunding.id)
      if (!refunded) return
      setRefunding(undefined)
      setSuccess({ kind: 'refund', credits: refunded.credits })
    } catch {
      // The catalog-backed failure remains in the confirmation dialog.
    }
  }

  const newest = [...purchases].sort(
    (left, right) => new Date(right.chargedAt).getTime() - new Date(left.chargedAt).getTime(),
  )

  return (
    <section className="grid gap-3">
      <Typography variant="title" as="h2">
        {t('purchases.heading')}
      </Typography>
      {success && (
        <Notice tone="success" role="status">
          {t(`purchases.success.${success.kind}`, { credits: success.credits })}
        </Notice>
      )}
      <div className="border-divider grid gap-3 rounded-xl border p-4">
        <Stepper
          label={t('purchases.amountLabel')}
          value={dollars}
          min={1}
          max={100}
          onChange={setDollars}
          decrementLabel={t('purchases.decrement')}
          incrementLabel={t('purchases.increment')}
          formatValue={(value) => t('purchases.dollars', { value })}
        />
        {quote && (
          <div className="grid gap-1">
            <Typography variant="fieldTitle">
              {t('purchases.quote', {
                credits: quote.credits,
                krw: formatNumber(quote.krw),
              })}
            </Typography>
            <Typography variant="meta" className="text-content-secondary">
              {t('purchases.rate', {
                date: quote.rateDate,
                rate: formatNumber(Number(quote.ratePerUsdE4) / 10_000),
              })}
            </Typography>
          </div>
        )}
        {quoteError && <FieldMessage>{t('purchases.quoteFailed')}</FieldMessage>}
        {!hasPaymentMethod && <FieldMessage>{t('purchases.paymentRequired')}</FieldMessage>}
        <Button
          variant="cta"
          disabled={!hasPaymentMethod || !quote || quotePending}
          onClick={() => {
            purchaseMutation.reset()
            setConfirmingPurchase(true)
          }}
        >
          {t('purchases.buy')}
        </Button>
      </div>

      {newest.length === 0 && (
        <Typography variant="body" className="text-content-secondary">
          {t('purchases.empty')}
        </Typography>
      )}
      {newest.length > 0 && (
        <ul className="divide-divider divide-y">
          {newest.map((purchase) => (
            <li key={purchase.id} className="grid gap-2 py-3 first:pt-1">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <Typography variant="label">
                  {t('purchases.row', {
                    credits: purchase.credits,
                    usd: (purchase.usdCents / 100).toFixed(2),
                    krw: formatNumber(purchase.krw),
                  })}
                </Typography>
                <Typography variant="meta" className="text-content-tertiary">
                  {formatDateTime(purchase.chargedAt)}
                </Typography>
              </div>
              {purchase.refundedAt ? (
                <Typography variant="meta" className="text-content-secondary">
                  {t('purchases.refunded', { date: formatDateTime(purchase.refundedAt) })}
                </Typography>
              ) : (
                purchase.refundable && (
                  <Button
                    variant="secondary"
                    className="w-fit"
                    onClick={() => {
                      refundMutation.reset()
                      setRefunding(purchase)
                    }}
                  >
                    {t('purchases.refund')}
                  </Button>
                )
              )}
            </li>
          ))}
        </ul>
      )}

      <Dialog
        open={confirmingPurchase}
        title={t('purchases.purchaseTitle')}
        confirmLabel={t('purchases.buy')}
        pending={purchaseMutation.isPending}
        onClose={() => setConfirmingPurchase(false)}
        onConfirm={() => void confirmPurchase()}
      >
        <span className="grid gap-2">
          <span>
            {t('purchases.purchaseDescription', {
              usd: dollars.toFixed(2),
              credits: quote?.credits ?? dollars * 100,
              krw: quote ? formatNumber(quote.krw) : '—',
            })}
          </span>
          <span>{t('purchases.consumption')}</span>
          {purchaseMutation.errorMessage && (
            <FieldMessage>{purchaseMutation.errorMessage}</FieldMessage>
          )}
        </span>
      </Dialog>

      <Dialog
        open={refunding !== undefined}
        title={t('purchases.refundTitle')}
        confirmLabel={t('purchases.refund')}
        pending={refundMutation.isPending}
        onClose={() => setRefunding(undefined)}
        onConfirm={() => void confirmRefund()}
      >
        <span className="grid gap-2">
          <span>{t('purchases.refundDescription')}</span>
          {refundMutation.errorMessage && (
            <FieldMessage>{refundMutation.errorMessage}</FieldMessage>
          )}
        </span>
      </Dialog>
    </section>
  )
}
