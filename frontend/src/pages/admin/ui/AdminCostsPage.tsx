import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useComparisonCosts,
  type ExperimentStageName,
  type LeaderboardWindowName,
} from '@/entities/model-experiment'
import { useExchangeRate } from '@/entities/plan'
import { formatNumber } from '@/shared/lib'
import { Badge, Button, Notice, SegmentedControl, Typography, typographyStyles } from '@/shared/ui'

/** The 비용·환율 tab of `/admin`: the operating figures customer screens never show, master
 *  included (QUOTA-65, QUOTA-68). Each figure is its own titled section, so the comparison cost
 *  of MODEL-39 joins below the rate as a sibling rather than a rearrangement. */
export function AdminCostsPage() {
  return (
    <div className="mt-8 grid gap-10">
      <ExchangeRateSection />
      <ComparisonCostsSection />
    </div>
  )
}

function ComparisonCostsSection() {
  const { t } = useTranslation(['plans', 'common'])
  const titleId = useId()
  const [stage, setStage] = useState<ExperimentStageName>('observe')
  const [window, setWindow] = useState<LeaderboardWindowName>('week')
  const { rows, isPending, isError, isFetching, refetch } = useComparisonCosts(stage, window)
  return (
    <section aria-labelledby={titleId} className="grid gap-3">
      <Typography variant="title" as="h2" id={titleId}>
        {t('admin.costs.comparisonHeading')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure">
        {t('admin.costs.comparisonDescription')}
      </Typography>
      <div className="grid gap-3 sm:flex sm:flex-wrap">
        <SegmentedControl
          value={stage}
          options={(['observe', 'write'] as const).map((value) => ({
            value,
            label: t(`admin.costs.stage.${value}`),
          }))}
          onChange={setStage}
          ariaLabel={t('admin.costs.stageAria')}
        />
        <SegmentedControl
          value={window}
          options={(['day', 'week', 'month'] as const).map((value) => ({
            value,
            label: t(`admin.costs.window.${value}`),
          }))}
          onChange={setWindow}
          ariaLabel={t('admin.costs.windowAria')}
        />
      </div>
      {isError && (
        <Notice tone="danger" role="alert">
          <span className="grid gap-2">
            <span>{t('admin.costs.comparisonLoadFailed')}</span>
            <Button
              variant="ghost"
              onClick={() => void refetch()}
              pending={isFetching}
              className="w-fit underline"
            >
              {t('action.retry', { ns: 'common' })}
            </Button>
          </span>
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary">
          {t('admin.costs.comparisonLoading')}
        </Typography>
      )}
      {!isError && !isPending && rows.length === 0 && (
        <Typography variant="body" className="text-content-tertiary">
          {t('admin.costs.comparisonEmpty')}
        </Typography>
      )}
      {!isError && !isPending && rows.length > 0 && (
        <ol className="divide-divider border-divider divide-y rounded-md border px-4">
          {rows.map((row) => (
            <li
              key={`${row.model.providerId}/${row.model.modelId}`}
              className="grid gap-1 py-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-baseline sm:gap-4"
            >
              <div className="min-w-0">
                <Typography variant="label" as="p" className="break-words">
                  {row.modelLabel || `${row.model.providerId}/${row.model.modelId}`}
                </Typography>
                <Typography variant="meta" as="p" className="text-content-secondary">
                  {t('admin.costs.comparisonCount', { count: row.evaluatedComparisons })}
                </Typography>
              </div>
              <Typography variant="label" as="p" className="tabular-nums">
                {row.costQuality === 'unavailable'
                  ? t('admin.costs.comparisonUnavailable')
                  : `${row.costQuality === 'reported' ? '' : '≈'}${formatMicrousd(row.totalCostMicrousd)}`}
                {row.costQuality === 'mixed' && (
                  <span className="text-content-secondary ml-2">
                    {t('admin.costs.comparisonMixed')}
                  </span>
                )}
              </Typography>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}

function formatMicrousd(value: bigint): string {
  const whole = value / 1_000_000n
  const fraction = (value % 1_000_000n).toString().padStart(6, '0')
  return `$${formatNumber(whole)}.${fraction}`
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
