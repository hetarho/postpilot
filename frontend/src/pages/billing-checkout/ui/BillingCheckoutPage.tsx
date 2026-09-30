import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { planLabel, PLANS, useMyPlan } from '@/entities/plan'
import {
  useChangeSubscription,
  useMyBilling,
  useQuote,
  useQuoteChange,
  useSubscribe,
  type BillingTerm,
} from '@/entities/subscription'
import { RegisterPaymentMethodButton } from '@/features/register-payment-method'
import { formatDateTime, formatNumber } from '@/shared/lib'
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
  const [paymentPending, setPaymentPending] = useState(false)
  const { myPlan, isPending: planPending, isError: planError } = useMyPlan()
  const { myBilling, isPending: billingPending, isError: billingError } = useMyBilling()
  const activeSubscription =
    myBilling?.subscription?.status === 'active' ? myBilling.subscription : undefined
  const isUpgrade = Boolean(
    activeSubscription?.plan &&
    tier &&
    PLANS.indexOf(tier) > PLANS.indexOf(activeSubscription.plan),
  )
  const selectedTerm = isUpgrade ? (activeSubscription?.term ?? term) : term
  const standardQuote = useQuote(tier, selectedTerm, !isUpgrade)
  const changeQuote = useQuoteChange(
    isUpgrade ? tier : undefined,
    isUpgrade ? selectedTerm : undefined,
  )
  const quote = isUpgrade ? changeQuote.quote : standardQuote.quote
  const quotePending = isUpgrade ? changeQuote.isPending : standardQuote.isPending
  const quoteError = isUpgrade ? changeQuote.isError : standardQuote.isError
  const subscribe = useSubscribe()
  const change = useChangeSubscription()
  const navigate = useNavigate()
  const offer = myPlan?.offers.find((candidate) => candidate.plan === tier)
  const invalid =
    tier === undefined ||
    (myPlan !== undefined &&
      (!offer ||
        myPlan.plan === 'master' ||
        (activeSubscription?.plan !== undefined &&
          PLANS.indexOf(tier) <= PLANS.indexOf(activeSubscription.plan))))
  const returnTo = tier ? `/billing/checkout?tier=${tier}` : '/billing/checkout'

  const submit = async () => {
    if (!tier || !myBilling?.paymentMethod) return
    try {
      if (isUpgrade) {
        const result = await change.changeSubscription(tier, selectedTerm, quote?.id)
        if (result.subscription?.status !== 'active') {
          setPaymentPending(true)
          return
        }
      } else {
        const result = await subscribe.subscribe(tier, selectedTerm)
        if (result.subscription?.status !== 'active') {
          setPaymentPending(true)
          return
        }
      }
      void navigate({
        to: '/billing',
        replace: true,
        state: (previous) => ({
          ...previous,
          billingSubscription: { tier: planLabel(tier), changed: isUpgrade },
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

      {paymentPending && (
        <Notice tone="info" role="status" className="mt-8">
          {t('checkout.paymentPending', { ns: 'billing' })}
        </Notice>
      )}
      {invalid && !paymentPending && (
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
              {t('checkout.benefits', {
                ns: 'billing',
                daily: offer.dailyCredits ?? 0,
                bonus: offer.monthlyBonus ?? 0,
                exports: offer.monthlyServerExports ?? 0,
              })}
            </Typography>
          </section>

          <section className="grid gap-3">
            <Typography variant="title" as="h2">
              {t('checkout.termHeading', { ns: 'billing' })}
            </Typography>
            {isUpgrade ? (
              <Typography variant="fieldTitle">
                {t(`checkout.term.${selectedTerm}`, { ns: 'billing' })}
              </Typography>
            ) : (
              <SegmentedControl
                value={term}
                options={terms.map((value) => ({
                  value,
                  label: t(`checkout.term.${value}`, { ns: 'billing' }),
                }))}
                onChange={setTerm}
                ariaLabel={t('checkout.termHeading', { ns: 'billing' })}
              />
            )}
            {selectedTerm === 'annual' && !isUpgrade && (
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
                    krw: formatNumber(quote.krw),
                  })}
                </Typography>
                <Typography variant="meta" className="text-content-secondary max-w-measure">
                  {t(
                    isUpgrade
                      ? 'checkout.proration'
                      : selectedTerm === 'annual'
                        ? 'checkout.annualUpfront'
                        : 'checkout.fixedPrice',
                    { ns: 'billing' },
                  )}
                </Typography>
                {isUpgrade && changeQuote.quote?.effectiveAt && (
                  <Typography variant="meta">
                    {t('checkout.upgradeTiming', {
                      ns: 'billing',
                      at: formatDateTime(changeQuote.quote.effectiveAt),
                      dailyAt: myPlan?.balance.dailyResetsAt
                        ? formatDateTime(myPlan.balance.dailyResetsAt)
                        : '—',
                    })}
                  </Typography>
                )}
                {isUpgrade && (
                  <Typography variant="meta" className="text-content-secondary max-w-measure">
                    {t('checkout.chargedNow', { ns: 'billing' })}
                  </Typography>
                )}
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

          {(subscribe.errorMessage || change.errorMessage) && (
            <Notice tone="danger" role="alert">
              {subscribe.errorMessage || change.errorMessage}
            </Notice>
          )}
          {myBilling.paymentMethod && quote && (
            <Button
              variant="cta"
              pending={subscribe.isPending || change.isPending}
              onClick={() => void submit()}
            >
              {t(isUpgrade ? 'checkout.upgradeSubmit' : 'checkout.submit', { ns: 'billing' })}
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
