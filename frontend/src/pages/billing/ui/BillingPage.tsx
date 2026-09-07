import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { planLabel } from '@/entities/plan'
import { useMyBilling, useQuote, type BillingEvent } from '@/entities/subscription'
import { RegisterPaymentMethodButton } from '@/features/register-payment-method'
import { RemovePaymentMethodButton } from '@/features/remove-payment-method'
import { BillingSubscriptionActions } from '@/features/manage-subscription'
import { formatDate, formatDateTime, formatNumber } from '@/shared/lib'
import { Notice, Typography, pageStyles, typographyStyles } from '@/shared/ui'

export function BillingPage() {
  const { t } = useTranslation(['billing', 'common'])
  const { myBilling, isPending, isError } = useMyBilling()
  const { quote } = useQuote(myBilling?.subscription?.plan, myBilling?.subscription?.term)
  const navigate = useNavigate()
  const initialRegistration = useRouterState({
    select: (state) => state.location.state.billingRegistration,
  })
  const initialSubscription = useRouterState({
    select: (state) => state.location.state.billingSubscription,
  })
  const [registration] = useState(initialRegistration)
  const [subscribed] = useState(initialSubscription)
  const noticeCleared = useRef(false)

  useEffect(() => {
    if ((!registration && !subscribed) || noticeCleared.current) return
    noticeCleared.current = true
    void navigate({
      to: '/billing',
      replace: true,
      state: (previous) => ({
        ...previous,
        billingRegistration: undefined,
        billingSubscription: undefined,
      }),
    })
  }, [navigate, registration, subscribed])

  return (
    <main className={pageStyles()}>
      <Typography variant="display">{t('title', { ns: 'billing' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('description', { ns: 'billing' })}
      </Typography>

      {registration && (
        <Notice tone="success" role="status" className="mt-6">
          {t(registration.bonusGranted ? 'registration.doneWithBonus' : 'registration.done', {
            ns: 'billing',
            label: registration.cardLabel,
            credits: 100,
          })}
        </Notice>
      )}
      {subscribed && (
        <Notice tone="success" role="status" className="mt-6">
          {t(subscribed.changed ? 'subscription.changed' : 'subscription.started', {
            ns: 'billing',
            tier: subscribed.tier,
          })}
        </Notice>
      )}

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          {t('loadFailed', { ns: 'billing' })}
        </Notice>
      )}
      {isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}
      {myBilling && (
        <div className="mt-10 grid gap-10">
          <section className="grid gap-2">
            <Typography variant="title" as="h2">
              {t('subscription.heading', { ns: 'billing' })}
            </Typography>
            {!myBilling.subscription && (
              <>
                <Typography variant="body" className="text-content-secondary">
                  {t('subscription.empty', { ns: 'billing' })}
                </Typography>
                <Link to="/plans" className={quietLink()}>
                  {t('subscription.choosePlans', { ns: 'billing' })}
                </Link>
              </>
            )}
            {myBilling.subscription && (
              <>
                <SubscriptionDetails subscription={myBilling.subscription} quote={quote} />
                <BillingSubscriptionActions subscription={myBilling.subscription} />
              </>
            )}
          </section>
          <section className="grid gap-2">
            <Typography variant="title" as="h2">
              {t('paymentMethod.heading', { ns: 'billing' })}
            </Typography>
            {!myBilling.paymentMethod && (
              <>
                <Typography variant="body" className="text-content-secondary">
                  {t('paymentMethod.empty', { ns: 'billing' })}
                </Typography>
                <RegisterPaymentMethodButton customerKey={myBilling.customerKey} />
              </>
            )}
            {myBilling.paymentMethod && (
              <>
                <dl className="grid gap-1">
                  <dt className={typographyStyles({ variant: 'label' })}>
                    {t('paymentMethod.card', { ns: 'billing' })}
                  </dt>
                  <dd className={typographyStyles({ variant: 'fieldTitle' })}>
                    {myBilling.paymentMethod.cardLabel}
                  </dd>
                  <dt className={typographyStyles({ variant: 'label', className: 'mt-2' })}>
                    {t('paymentMethod.registeredAt', { ns: 'billing' })}
                  </dt>
                  <dd className={typographyStyles({ variant: 'body' })}>
                    {formatDate(myBilling.paymentMethod.registeredAt)}
                  </dd>
                </dl>
                <div className="mt-2 flex flex-wrap gap-2">
                  <RegisterPaymentMethodButton customerKey={myBilling.customerKey} registered />
                  <RemovePaymentMethodButton />
                </div>
              </>
            )}
          </section>
          <section className="grid gap-2">
            <Typography variant="title" as="h2">
              {t('history.heading', { ns: 'billing' })}
            </Typography>
            {myBilling.history.length === 0 && (
              <Typography variant="body" className="text-content-secondary">
                {t('history.empty', { ns: 'billing' })}
              </Typography>
            )}
            {myBilling.history.length > 0 && <BillingHistory events={myBilling.history} />}
          </section>
          <section className="grid gap-2">
            <Typography variant="title" as="h2">
              {t('purchases.heading', { ns: 'billing' })}
            </Typography>
            {myBilling.purchases.length === 0 && (
              <Typography variant="body" className="text-content-secondary">
                {t('purchases.empty', { ns: 'billing' })}
              </Typography>
            )}
          </section>
        </div>
      )}
    </main>
  )
}

function SubscriptionDetails({
  subscription,
  quote,
}: {
  subscription: NonNullable<ReturnType<typeof useMyBilling>['myBilling']>['subscription']
  quote: ReturnType<typeof useQuote>['quote']
}) {
  const { t } = useTranslation('billing')
  if (!subscription) return null
  const anchorDay = new Intl.DateTimeFormat('en', {
    day: 'numeric',
    timeZone: 'Asia/Seoul',
  }).format(new Date(subscription.anchorAt))
  return (
    <dl className="grid gap-1">
      <dt className={typographyStyles({ variant: 'label' })}>{t('subscription.tier')}</dt>
      <dd className={typographyStyles({ variant: 'fieldTitle' })}>
        {planLabel(subscription.plan)}
      </dd>
      <dt className={typographyStyles({ variant: 'label', className: 'mt-2' })}>
        {t('subscription.termLabel')}
      </dt>
      <dd className={typographyStyles({ variant: 'body' })}>
        {t(`subscription.term.${subscription.term ?? 'monthly'}`)}
      </dd>
      <dt className={typographyStyles({ variant: 'label', className: 'mt-2' })}>
        {t('subscription.anchor')}
      </dt>
      <dd className={typographyStyles({ variant: 'body' })}>
        {t('subscription.anchorDay', { day: anchorDay })}
      </dd>
      <dt className={typographyStyles({ variant: 'label', className: 'mt-2' })}>
        {t('subscription.nextCharge')}
      </dt>
      <dd className={typographyStyles({ variant: 'body' })}>
        {formatDate(subscription.termEnd)}
        {quote && (
          <Typography variant="meta" as="span" className="text-content-secondary ml-2">
            {t('subscription.movingQuote', { krw: formatNumber(quote.krw) })}
          </Typography>
        )}
      </dd>
    </dl>
  )
}

function BillingHistory({ events }: { events: BillingEvent[] }) {
  const { t } = useTranslation('billing')
  const newest = [...events].sort((left, right) => {
    const timeDifference = new Date(right.createdAt).getTime() - new Date(left.createdAt).getTime()
    if (timeDifference !== 0) return timeDifference
    return right.id > left.id ? 1 : right.id < left.id ? -1 : 0
  })
  return (
    <ul className="divide-divider divide-y">
      {newest.map((event) => (
        <li key={event.id.toString()} className="grid gap-1 py-3 first:pt-1">
          <div className="flex flex-wrap items-baseline justify-between gap-2">
            <Typography variant="label">{t(eventKindKey(event.kind))}</Typography>
            <Typography variant="meta" className="text-content-tertiary">
              {formatDateTime(event.createdAt)}
            </Typography>
          </div>
          {(event.usdCents > 0 || event.krw > 0n) && (
            <Typography variant="body">
              {t('history.amount', {
                usd: (event.usdCents / 100).toFixed(2),
                krw: formatNumber(event.krw),
              })}
            </Typography>
          )}
          {event.credits > 0 && (
            <Typography variant="body">
              {t('history.credits', { credits: event.credits })}
            </Typography>
          )}
          {event.krwPerUsdE4 > 0n && (
            <Typography variant="meta" className="text-content-secondary">
              {t('history.rate', {
                rate: formatNumber(Number(event.krwPerUsdE4) / 10_000),
                date: event.rateDate,
              })}
            </Typography>
          )}
        </li>
      ))}
    </ul>
  )
}

function eventKindKey(kind: string) {
  switch (kind) {
    case 'charge':
      return 'history.kind.charge' as const
    case 'charge_failed':
      return 'history.kind.charge_failed' as const
    case 'tier_change':
      return 'history.kind.tier_change' as const
    case 'grant':
      return 'history.kind.grant' as const
    case 'renewal_failed':
      return 'history.kind.renewal_failed' as const
    case 'cancelled':
      return 'history.kind.cancelled' as const
    case 'cancel_scheduled':
      return 'history.kind.cancel_scheduled' as const
    case 'change_scheduled':
      return 'history.kind.change_scheduled' as const
    case 'method_registered':
      return 'history.kind.method_registered' as const
    default:
      return 'history.kind.other' as const
  }
}

function quietLink() {
  return typographyStyles({
    variant: 'label',
    className:
      'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 w-fit items-center underline',
  })
}
