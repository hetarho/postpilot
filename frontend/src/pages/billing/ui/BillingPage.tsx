import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useMyBilling } from '@/entities/subscription'
import { Notice, Typography, pageStyles, typographyStyles } from '@/shared/ui'

export function BillingPage() {
  const { t } = useTranslation(['billing', 'common'])
  const { myBilling, isPending, isError } = useMyBilling()

  return (
    <main className={pageStyles()}>
      <Typography variant="display">{t('title', { ns: 'billing' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('description', { ns: 'billing' })}
      </Typography>

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
              <Typography variant="body" className="text-content-secondary">
                {t('paymentMethod.empty', { ns: 'billing' })}
              </Typography>
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
