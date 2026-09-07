import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { planLabel, useMyPlan } from '@/entities/plan'
import { useMyBilling, useQuote, useSubscribe, type BillingTerm } from '@/entities/subscription'
import { RegisterPaymentMethodButton } from '@/features/register-payment-method'
import { formatNumber } from '@/shared/lib'
import {
  Button,
  Notice,
  SegmentedControl,
  Typography,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

const terms = ['monthly', 'annual'] as const

/** BILLING's committing surface. The plan ladder only chooses a rung; this page owns every
 * value that can move before money is charged: term, server quote and registered card. */
export function BillingCheckoutPage() {
  const { t } = useTranslation(['billing', 'common'])
  const { tier } = useSearch({ from: '/authenticated/billing/checkout' })
  const [term, setTerm] = useState<BillingTerm>('monthly')
  const { myPlan, isPending: planPending, isError: planError } = useMyPlan()
  const { myBilling, isPending: billingPending, isError: billingError } = useMyBilling()
  const { quote, isPending: quotePending, isError: quoteError } = useQuote(tier, term)
  const subscribe = useSubscribe()
  const navigate = useNavigate()
  const offer = myPlan?.offers.find((candidate) => candidate.plan === tier)
  const invalid =
    tier === undefined || (myPlan !== undefined && (!offer || myPlan.plan === 'master'))
  const returnTo = tier ? `/billing/checkout?tier=${tier}` : '/billing/checkout'

  const submit = async () => {
    if (!tier || !myBilling?.paymentMethod) return
    try {
      await subscribe.subscribe(tier, term)
      void navigate({
        to: '/billing',
        replace: true,
        state: (previous) => ({
          ...previous,
          billingSubscription: { tier: planLabel(tier) },
        }),
      })
    } catch {
      // The mutation exposes the catalog-backed refusal beside the committing action.
    }
  }

  return (
    <main className={pageStyles()}>
      <Typography variant="display">{t('checkout.title', { ns: 'billing' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('checkout.description', { ns: 'billing' })}
      </Typography>

      {invalid && (
        <Notice tone="danger" role="alert" className="mt-8">
          {t('checkout.invalid', { ns: 'billing' })}
        </Notice>
      )}
      {(planError || billingError || quoteError) && !invalid && (
        <Notice tone="danger" role="alert" className="mt-8">
          {t('checkout.loadFailed', { ns: 'billing' })}
        </Notice>
      )}
      {(planPending || billingPending) && !invalid && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!invalid && offer && myBilling && (
        <div className="mt-8 grid gap-8">
          <section className="grid gap-2">
            <Typography variant="title" as="h2">
              {planLabel(offer.plan)}
            </Typography>
            <Typography variant="body" className="text-content-secondary">
              {t('checkout.monthlyGrant', { ns: 'billing', credits: offer.monthlyCredits })}
            </Typography>
          </section>

          <section className="grid gap-3">
            <Typography variant="title" as="h2">
              {t('checkout.termHeading', { ns: 'billing' })}
            </Typography>
            <SegmentedControl
              value={term}
              options={terms.map((value) => ({
                value,
                label: t(`checkout.term.${value}`, { ns: 'billing' }),
              }))}
              onChange={setTerm}
              ariaLabel={t('checkout.termHeading', { ns: 'billing' })}
            />
            {term === 'annual' && (
              <Typography variant="meta" className="text-content-secondary">
                {t('checkout.annualValue', { ns: 'billing' })}
              </Typography>
            )}
          </section>

          <section className="grid gap-2" aria-busy={quotePending}>
            <Typography variant="title" as="h2">
              {t('checkout.priceHeading', { ns: 'billing' })}
            </Typography>
            {quote && (
              <>
                <Typography variant="fieldTitle">
                  {t('checkout.price', {
                    ns: 'billing',
                    usd: (quote.usdCents / 100).toFixed(2),
                    krw: formatNumber(quote.krw),
                  })}
                </Typography>
                <Typography variant="meta" className="text-content-secondary max-w-measure">
                  {t('checkout.rate', {
                    ns: 'billing',
                    rate: formatNumber(Number(quote.ratePerUsdE4) / 10_000),
                    date: quote.rateDate,
                  })}
                </Typography>
                <Typography variant="meta" className="text-content-secondary max-w-measure">
                  {t('checkout.moving', { ns: 'billing' })}
                </Typography>
              </>
            )}
          </section>

          <section className="grid gap-2">
            <Typography variant="title" as="h2">
              {t('paymentMethod.heading', { ns: 'billing' })}
            </Typography>
            {myBilling.paymentMethod ? (
              <Typography variant="fieldTitle">{myBilling.paymentMethod.cardLabel}</Typography>
            ) : (
              <>
                <Notice tone="info" role="status">
                  {t('checkout.paymentRequired', { ns: 'billing' })}
                </Notice>
                <RegisterPaymentMethodButton
                  customerKey={myBilling.customerKey}
                  returnTo={returnTo}
                />
              </>
            )}
          </section>

          {subscribe.errorMessage && (
            <Notice tone="danger" role="alert">
              {subscribe.errorMessage}
            </Notice>
          )}
          {myBilling.paymentMethod && quote && (
            <Button variant="cta" pending={subscribe.isPending} onClick={() => void submit()}>
              {t('checkout.submit', { ns: 'billing' })}
            </Button>
          )}
          <Link
            to="/plans"
            className={typographyStyles({
              variant: 'label',
              className: 'text-link-fg hover:text-link-fg-hover w-fit underline',
            })}
          >
            {t('checkout.back', { ns: 'billing' })}
          </Link>
        </div>
      )}
    </main>
  )
}
