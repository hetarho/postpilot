import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { useExchangeRate } from '@/entities/plan'
import { formatNumber } from '@/shared/lib'
import { Badge, Button, Notice, Typography, typographyStyles } from '@/shared/ui'

/** The 비용·환율 tab of `/admin`: the operating figures customer screens never show, master
 *  included (QUOTA-65, QUOTA-68). Each figure is its own titled section, so the comparison cost
 *  of MODEL-39 joins below the rate as a sibling rather than a rearrangement. */
export function AdminCostsPage() {
  return (
    <div className="mt-8 grid gap-10">
      <ExchangeRateSection />
    </div>
  )
}

/** The rate a paid job admitted now would be priced at (QUOTA-59). A missing rate is a state
 *  of the product, not of this read, so it is said as what it stops rather than as an error. */
function ExchangeRateSection() {
  const { t } = useTranslation(['plans', 'common'])
  const titleId = useId()
  const { exchangeRate, isPending, isError, isFetching, refetch } = useExchangeRate()
  const rate = exchangeRate?.rate

  return (
    <section aria-labelledby={titleId} className="grid gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Typography variant="title" as="h2" id={titleId}>
          {t('admin.costs.rateHeading')}
        </Typography>
        {rate?.temporary && <Badge tone="warning">{t('admin.costs.temporary')}</Badge>}
      </div>
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('admin.costs.rateDescription')}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert">
          <span className="grid gap-2">
            <span>{t('admin.costs.loadFailed')}</span>
            <Button
              variant="ghost"
              onClick={refetch}
              pending={isFetching}
              className="text-notice-danger-fg w-fit underline"
            >
              {t('action.retry', { ns: 'common' })}
            </Button>
          </span>
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary">
          {t('admin.costs.loading')}
        </Typography>
      )}
      {!isError && exchangeRate?.unavailable && (
        <Notice tone="info" role="status">
          {t('admin.costs.unavailable')}
        </Notice>
      )}
      {!isError && rate && (
        <>
          <dl className="bg-surface-raised grid gap-x-6 gap-y-3 rounded-md p-4 sm:grid-cols-2">
            {(
              [
                ['source', rate.source],
                ['publicationDate', rate.publicationDate],
                ['reference', t('admin.costs.perUsd', { value: formatNumber(rate.reference) })],
                ['applied', t('admin.costs.perUsd', { value: formatNumber(rate.applied) })],
              ] as const
            ).map(([key, value]) => (
              <div key={key} className="min-w-0">
                <dt className={typographyStyles({ variant: 'label' })}>
                  {t(`admin.costs.${key}`)}
                </dt>
                <dd
                  className={typographyStyles({
                    variant: 'fieldTitle',
                    className: 'mt-1 break-words tabular-nums',
                  })}
                >
                  {value}
                </dd>
              </div>
            ))}
          </dl>
          {rate.temporary && (
            <Typography variant="meta" className="text-content-secondary">
              {t('admin.costs.temporaryNote')}
            </Typography>
          )}
        </>
      )}
    </section>
  )
}
