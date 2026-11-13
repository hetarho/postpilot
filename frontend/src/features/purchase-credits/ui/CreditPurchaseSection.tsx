import { useState } from 'react'
import { useMyPlan } from '@/entities/plan'
import { useTranslation } from 'react-i18next'
import { usePurchaseCredits, useQuotePack, type Purchase } from '@/entities/subscription'
import { formatDateTime, formatNumber } from '@/shared/lib'
import { Button, Dialog, FieldMessage, Notice, RadioGroup, Typography } from '@/shared/ui'

export function CreditPurchaseSection({
  hasPaymentMethod,
  purchases,
  activePaid,
}: {
  hasPaymentMethod: boolean
  purchases: Purchase[]
  activePaid: boolean
}) {
  const { t } = useTranslation('billing')
  const { myPlan } = useMyPlan()
  const packs = myPlan?.creditPacks ?? []
  const [packId, setPackId] = useState('')
  const selectedPack = packs.find((pack) => pack.id === packId) ?? packs[0]
  const paid = activePaid && ['light', 'basic', 'pro', 'max'].includes(myPlan?.plan ?? '')
  const [confirmingPurchase, setConfirmingPurchase] = useState(false)
  const [success, setSuccess] = useState<{ kind: 'purchase'; credits: number }>()
  const {
    quote,
    isPending: quotePending,
    isError: quoteError,
  } = useQuotePack(paid ? (selectedPack?.id ?? '') : '')
  const purchaseMutation = usePurchaseCredits()

  const confirmPurchase = async () => {
    try {
      const purchased = await purchaseMutation.purchasePack(selectedPack!.id)
      if (!purchased) return
      setConfirmingPurchase(false)
      setSuccess({ kind: 'purchase', credits: purchased.credits })
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
      <div className="bg-card-well grid min-w-0 gap-3 rounded-lg p-4">
        <Typography variant="fieldTitle" as="h3">
          {t('purchases.amountLabel')}
        </Typography>
        <RadioGroup
          label={t('purchases.amountLabel')}
          value={selectedPack?.id ?? ''}
          disabled={!paid}
          onChange={setPackId}
          options={packs.map((pack) => ({
            value: pack.id,
            label: t('purchases.pack', { credits: pack.credits, krw: formatNumber(pack.priceKrw) }),
          }))}
        />
        {quote && (
          <Typography variant="fieldTitle">
            {t('purchases.quote', { credits: quote.credits, krw: formatNumber(quote.krw) })}
          </Typography>
        )}
        {!paid && <FieldMessage>{t('purchases.paidRequired')}</FieldMessage>}
        {quoteError && <FieldMessage>{t('purchases.quoteFailed')}</FieldMessage>}
        {!hasPaymentMethod && <FieldMessage>{t('purchases.paymentRequired')}</FieldMessage>}
        <Button
          variant="cta"
          disabled={!paid || !hasPaymentMethod || !quote || quotePending}
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
              ) : null}
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
              credits: quote?.credits ?? selectedPack?.credits ?? 0,
              krw: quote ? formatNumber(quote.krw) : '—',
            })}
          </span>
          <span>{t('purchases.consumption')}</span>
          {purchaseMutation.errorMessage && (
            <FieldMessage>{purchaseMutation.errorMessage}</FieldMessage>
          )}
        </span>
      </Dialog>
    </section>
  )
}
