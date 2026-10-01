import { useTranslation } from 'react-i18next'
import { planLabel, type EstimatorComboName, type PlanName } from '@/entities/plan'
import { PromoFrame, Typography } from '@/shared/ui'
import { PROMO_COUNT_UP_MS } from '../config'
import { useCountUp } from '../model/useCountUp'

/** One production figure: a count, or why there is none. A missing figure is said, never
 *  replaced by another level's count or a zero that reads as "free" (QUOTA-56). */
export type PlanComparisonFigure = { count: number } | { unavailable: 'pricing' | 'clip' | 'rate' }

/** One tier in the comparison below the cards (QUOTA-36, QUOTA-41). A paid tier names the one
 *  level its counts use; free carries no counts, only provider-limited availability. */
export interface PlanComparisonRow {
  plan: PlanName
  level?: EstimatorComboName
  posts?: PlanComparisonFigure
  clips?: PlanComparisonFigure
  /** The post figure's credits and where they came from (QUOTA-64). */
  perPost?: { credits: number; basis: 'recent' | 'estimate' }
}

/** Every tier's monthly post count and AI-clip count side by side, one row per tier. The
 *  caller states the assumption, the clip conditions and the caveats around it. */
export function PlanComparison({ rows }: { rows: readonly PlanComparisonRow[] }) {
  const { t } = useTranslation('plans')
  return (
    <PromoFrame density="compact">
      <ul className="divide-divider divide-y">
        {rows.map((row) => (
          <li
            key={row.plan}
            className="grid min-w-0 gap-2 py-3 first:pt-0 last:pb-0 sm:grid-cols-3 sm:items-start sm:gap-4"
          >
            <div className="min-w-0">
              <Typography variant="label" as="h3" className="text-content-primary">
                {planLabel(row.plan)}
              </Typography>
              <Typography variant="meta" className="text-content-secondary">
                {row.level
                  ? t('comparison.level', { level: t(`estimator.combos.${row.level}`) })
                  : t('compare.freeModels')}
              </Typography>
            </div>
            {row.level ? (
              <dl className="grid min-w-0 grid-cols-2 gap-3 sm:col-span-2">
                <Figure label={t('comparison.posts')} figure={row.posts} perPost={row.perPost} />
                <Figure label={t('comparison.clips')} figure={row.clips} />
              </dl>
            ) : (
              <Typography variant="body" className="text-content-secondary sm:col-span-2">
                {t('compare.freeLimits')}
              </Typography>
            )}
          </li>
        ))}
      </ul>
    </PromoFrame>
  )
}

function Figure({
  label,
  figure,
  perPost,
}: {
  label: string
  figure: PlanComparisonFigure | undefined
  perPost?: PlanComparisonRow['perPost']
}) {
  const { t } = useTranslation('plans')
  const count = figure && 'count' in figure ? figure.count : undefined
  const shown = useCountUp(count ?? 0, PROMO_COUNT_UP_MS)
  return (
    <div className="min-w-0">
      <Typography variant="meta" as="dt" className="text-content-secondary">
        {label}
      </Typography>
      {count === undefined ? (
        <Typography variant="meta" as="dd" className="text-content-secondary">
          {figure && 'unavailable' in figure && figure.unavailable === 'clip'
            ? t('estimator.clipUnavailable')
            : figure && 'unavailable' in figure && figure.unavailable === 'rate'
              ? t('comparison.rateUnavailable')
              : t('estimator.unavailable')}
        </Typography>
      ) : (
        <Typography variant="fieldTitle" as="dd" className="text-content-primary tabular-nums">
          {count === 0 ? t('estimator.tooSmall') : t('comparison.count', { count: shown })}
        </Typography>
      )}
      {perPost && count !== undefined && (
        <Typography variant="meta" as="dd" className="text-content-tertiary tabular-nums">
          {t('estimator.perPost', { credits: perPost.credits })}
          {' · '}
          {t(perPost.basis === 'recent' ? 'estimator.basisRecent' : 'estimator.basisEstimate')}
        </Typography>
      )}
    </div>
  )
}
