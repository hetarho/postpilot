import { Link, useNavigate, useRouterState } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMyBilling } from '@/entities/subscription'
import { RegisterPaymentMethodButton } from '@/features/register-payment-method'
import { RemovePaymentMethodButton } from '@/features/remove-payment-method'
import { formatDate } from '@/shared/lib'
import { Notice, Typography, pageStyles, typographyStyles } from '@/shared/ui'

export function BillingPage() {
  const { t } = useTranslation(['billing', 'common'])
  const { myBilling, isPending, isError } = useMyBilling()
  const navigate = useNavigate()
  const initialRegistration = useRouterState({
    select: (state) => state.location.state.billingRegistration,
  })
  const [registration] = useState(initialRegistration)
  const noticeCleared = useRef(false)

  useEffect(() => {
    if (!registration || noticeCleared.current) return
    noticeCleared.current = true
    void navigate({
      to: '/billing',
      replace: true,
      state: (previous) => ({ ...previous, billingRegistration: undefined }),
    })
  }, [navigate, registration])

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

function quietLink() {
  return typographyStyles({
    variant: 'label',
    className:
      'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 w-fit items-center underline',
  })
}
