import type { ComponentType, ReactNode } from 'react'
import { clsx } from 'clsx'
import { Crown, Gift, Leaf, Rocket, Sparkles, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  planLabel,
  subscriptionBonus,
  type EstimatorComboName,
  type PlanName,
  type PlanOffer,
} from '@/entities/plan'
import { PROMO_COUNT_UP_MS, PROMO_RISE_STAGGER_MS } from '../config'
import { Badge, PromoFrame, PromoText, Typography } from '@/shared/ui'
import { useCountUp } from '../model/useCountUp'

/** Each rung's glyph. Presentation only — the ladder's order and figures are the caller's, and
 *  the icon is how a card is told apart at a glance before its name is read. The operator tier
 *  is never on offer, but the type includes it so a future rung cannot arrive without one. */
const TIER_ICON: Record<PlanName, ComponentType<{ className?: string }>> = {
  free: Leaf,
  basic: Zap,
  pro: Sparkles,
  max: Rocket,
  master: Crown,
}

/** Shared comparison cards. The caller owns the figures, estimates and billing actions.
 *  Phone summaries put name and price together; wider layouts keep four peer columns. */
export function PlanLadder({
  offers,
  currentPlan,
  estimates,
  action,
  headingLevel = 'h2',
  className,
}: {
  offers: readonly PlanOffer[]
  /** The rung the reader is on, which is named and offered no action. */
  currentPlan?: PlanName
  /** Model-level capacities for the chosen conditions. Omitted on the public about page. */
  estimates?: (offer: PlanOffer) => readonly PlanEstimate[]
  /** The rung's action, or nothing. The caller owns what an action is and where it leads. */
  action?: (offer: PlanOffer) => ReactNode
  /** The outline level the tier names take: `h2` on `/plans`, whose title is the page's `h1`;
   *  `h3` under a section title elsewhere. */
  headingLevel?: 'h2' | 'h3'
  className?: string
}) {
  return (
    <ul className={clsx('grid min-w-0 gap-3 md:grid-cols-2 lg:grid-cols-4 lg:gap-6', className)}>
      {offers.map((offer, index) => (
        <li key={offer.plan} className="min-w-0">
          <PlanCard
            offer={offer}
            index={index}
            current={offer.plan === currentPlan}
            estimates={estimates?.(offer)}
            action={action?.(offer)}
            headingLevel={headingLevel}
          />
        </li>
      ))}
    </ul>
  )
}

/** A compact summary keeps every decision-making figure visible, including all four
 *  estimates. Shared model access and renewal copy belong outside the peer cards. */
function PlanCard({
  offer,
  index,
  current,
  estimates,
  action,
  headingLevel,
}: {
  offer: PlanOffer
  /** The rung's position, which sets how long after the first it rises into place. */
  index: number
  current: boolean
  estimates: readonly PlanEstimate[] | undefined
  action: ReactNode
  headingLevel: 'h2' | 'h3'
}) {
  const { t } = useTranslation('plans')
  const Icon = TIER_ICON[offer.plan ?? 'free']
  const priced = offer.priceUsdCents > 0
  const bonus = subscriptionBonus(offer)

  return (
    <PromoFrame
      density="compact"
      marked={offer.recommended}
      // The desk has room for the recommendation to stand a step taller than its neighbours;
      // a phone stack does not, and the halo already says it there. `z-10` lets the lifted card
      // and its halo overlap the rungs beside it instead of being cut by their edges.
      className={clsx('animate-rise', offer.recommended && 'lg:z-10 lg:scale-105')}
      style={{ animationDelay: `${index * PROMO_RISE_STAGGER_MS}ms` }}
    >
      <div className="flex h-full min-w-0 flex-col">
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 md:items-start">
          <div className="flex min-w-0 flex-wrap items-center gap-2">
            <Icon aria-hidden="true" className="text-badge-accent-fg size-5 shrink-0" />
            <Typography variant="title" as={headingLevel}>
              {planLabel(offer.plan)}
            </Typography>
            {current && <Badge tone="accent">{t('compare.current')}</Badge>}
            {!current && offer.recommended && (
              <Badge tone="accent">{t('compare.recommended')}</Badge>
            )}
          </div>
          <p className="flex flex-wrap items-baseline gap-x-1 md:w-full">
            <PromoText variant="hero" as="span" className="tabular-nums">
              {priced
                ? t('compare.priceUsd', { usd: (offer.priceUsdCents / 100).toFixed(0) })
                : t('compare.priceFree')}
            </PromoText>
            {priced && (
              <Typography variant="meta" as="span" className="text-content-secondary">
                {t('compare.perMonth')}
              </Typography>
            )}
          </p>
        </div>
        <Typography variant="body" className="text-content-secondary mt-1">
          {t('compare.monthlyCredits', { credits: offer.monthlyCredits })}
        </Typography>
        {bonus && (
          <div className="text-badge-accent-fg mt-1 flex flex-wrap items-center gap-x-2 gap-y-1">
            <Typography
              variant="meta"
              as="span"
              className="text-badge-accent-fg inline-flex items-center gap-1"
            >
              <Gift aria-hidden="true" className="size-3.5 shrink-0" />
              {t('benefits.extra', { credits: bonus.credits })}
            </Typography>
            <Typography variant="meta" as="span" className="text-badge-accent-fg">
              {t('benefits.percent', { percent: bonus.percent })}
            </Typography>
          </div>
        )}
        {estimates && (
          <dl className="border-divider mt-3 grid grid-cols-2 gap-x-3 gap-y-2 border-t pt-3 md:grid-cols-1">
            {estimates.map((estimate) => (
              <ModelEstimate key={`${estimate.kind}-${estimate.level}`} estimate={estimate} />
            ))}
          </dl>
        )}
        {action && <div className="mt-auto pt-3">{action}</div>}
      </div>
    </PromoFrame>
  )
}

interface PlanEstimate {
  level: EstimatorComboName
  kind: 'blog' | 'clip'
  count: number | undefined
}

function ModelEstimate({ estimate }: { estimate: PlanEstimate }) {
  const { t } = useTranslation('plans')
  const shown = useCountUp(estimate.count ?? 0, PROMO_COUNT_UP_MS)
  return (
    <div className="min-w-0">
      <Typography variant="meta" as="dt" className="text-content-secondary">
        {t('estimator.model', { level: t(`estimator.combos.${estimate.level}`) })}
      </Typography>
      <Typography variant="label" as="dd" className="text-badge-accent-fg tabular-nums">
        {estimate.count === undefined
          ? t(estimate.kind === 'clip' ? 'estimator.clipUnavailable' : 'estimator.unavailable')
          : estimate.count === 0
            ? t('estimator.tooSmall')
            : t(estimate.kind === 'clip' ? 'estimator.clips' : 'estimator.posts', { count: shown })}
      </Typography>
    </div>
  )
}
