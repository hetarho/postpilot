import type { ComponentType, ReactNode } from 'react'
import { clsx } from 'clsx'
import { Check, Crown, Gift, Leaf, Rocket, Sparkles, Zap } from 'lucide-react'
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

/** The plan ladder: one promotional card per offered rung (THEME-37, QUOTA-28).
 *
 *  It renders what it is handed and decides nothing: `/plans` feeds it the ladder GetMyPlan
 *  published together with each rung's post estimate and action, and `/about` feeds it the
 *  static code-owned ladder with neither (MARKETING-5). Every figure is therefore the caller's —
 *  a grant or a price kept here would be a second source of truth that goes stale on its own.
 *
 *  Stacked on a phone and side by side upward: four rungs at `md:` leave ~180px a card, which
 *  wraps every figure onto its own ragged line, so the shape changes at `md:` in two columns and
 *  only reaches four on the desk (THEME-14). The desk row breathes wider than the phone stack —
 *  the marked rung stands a step taller there and its halo needs the room between neighbours
 *  rather than their edges. */
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
    <ul className={clsx('grid gap-4 md:grid-cols-2 lg:grid-cols-4 lg:gap-6', className)}>
      {offers.map((offer, index) => (
        <li key={offer.plan}>
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

/** One rung. A card here is the case §1.4 allows: an item in a grid of peers, whose contents
 *  read as one unit and apart from the rung above and below it.
 *
 *  The card is a column: glyph and status chip on top, the tier's name, the price as the one
 *  `hero` figure this design language has — gradient ink, since the thing being compared
 *  outranks the page's own title on a surface whose job is to be chosen from — what the grant
 *  buys on the accent plate, the grant itself, and the action pinned to the bottom so four
 *  buttons share a baseline.
 *
 *  Every rung wears the promotional stroke and the spotlight; the recommended one wears the
 *  stroke wider, with the halo and — on the desk — a step of scale (THEME-37). The badge, not
 *  the glow, is what says which rung is recommended, and the flag comes from the caller's
 *  ladder so a second marked card is impossible by construction rather than by review. */
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
      marked={offer.recommended}
      // The desk has room for the recommendation to stand a step taller than its neighbours;
      // a phone stack does not, and the halo already says it there. `z-10` lets the lifted card
      // and its halo overlap the rungs beside it instead of being cut by their edges.
      className={clsx('animate-rise', offer.recommended && 'lg:z-10 lg:scale-105')}
      style={{ animationDelay: `${index * PROMO_RISE_STAGGER_MS}ms` }}
    >
      <div className="flex h-full flex-col">
        {/* `flex-wrap`: the glyph plate is `shrink-0` and a Badge never wraps its label (both by
            design), so on one line the pair fixes the card's min-content — and a grid item's
            automatic minimum size then pushes the whole column past a 320px viewport once the
            reader's text size doubles. Wrapping is the only give in the row; it costs nothing at
            ordinary sizes, where the pair has always fitted. */}
        <div className="flex flex-wrap items-start justify-between gap-2">
          {/* The glyph sits on the accent's quiet plate — the same pair the status chip beside
              it wears — so the card opens with one violet mark and nothing competing for it. */}
          <span
            aria-hidden="true"
            className="bg-badge-accent-bg text-badge-accent-fg inline-flex size-12 shrink-0 items-center justify-center rounded-full"
          >
            <Icon className="size-6" />
          </span>
          {current && <Badge tone="accent">{t('compare.current')}</Badge>}
          {!current && offer.recommended && <Badge tone="accent">{t('compare.recommended')}</Badge>}
        </div>
        <Typography variant="title" as={headingLevel} className="mt-4">
          {planLabel(offer.plan)}
        </Typography>
        <p className="mt-1 flex flex-wrap items-baseline gap-x-1.5">
          <PromoText variant="hero" as="span" className="tabular-nums">
            {priced
              ? t('compare.priceUsd', { usd: (offer.priceUsdCents / 100).toFixed(0) })
              : t('compare.priceFree')}
          </PromoText>
          {priced && (
            <Typography variant="label" as="span">
              {t('compare.perMonth')}
            </Typography>
          )}
        </p>
        {estimates && (
          <dl className="border-divider divide-divider mt-5 divide-y border-y">
            {estimates.map((estimate) => (
              <ModelEstimate key={`${estimate.kind}-${estimate.level}`} estimate={estimate} />
            ))}
          </dl>
        )}
        <Typography variant="body" className="text-content-secondary mt-3 block">
          {t('compare.monthlyCredits', { credits: offer.monthlyCredits })}
        </Typography>
        {bonus && (
          <div className="bg-badge-accent-bg text-badge-accent-fg mt-4 rounded-lg px-3 py-3">
            <Typography variant="fieldTitle" as="p" className="flex items-center gap-2">
              <Gift aria-hidden="true" className="size-4 shrink-0" />
              {t('benefits.extra', { credits: bonus.credits })}
            </Typography>
            <Typography variant="body" className="mt-1">
              {t('benefits.percent', { percent: bonus.percent })}
            </Typography>
          </div>
        )}
        <div className="mt-5 space-y-2">
          <Typography variant="body" className="text-content-secondary flex items-center gap-2">
            <Check aria-hidden="true" className="text-badge-accent-fg size-4 shrink-0" />
            {t('benefits.models')}
          </Typography>
          <Typography variant="body" className="text-content-secondary flex items-center gap-2">
            <Check aria-hidden="true" className="text-badge-accent-fg size-4 shrink-0" />
            {t(priced ? 'benefits.renewal' : 'benefits.free')}
          </Typography>
        </div>
        {action && <div className="mt-auto pt-5">{action}</div>}
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
    <div className="py-3">
      <Typography variant="meta" as="dt" className="text-content-secondary">
        {t('estimator.model', { level: t(`estimator.combos.${estimate.level}`) })}
      </Typography>
      <Typography variant="label" as="dd" className="text-badge-accent-fg mt-1 tabular-nums">
        {estimate.count === undefined
          ? t(estimate.kind === 'clip' ? 'estimator.clipUnavailable' : 'estimator.unavailable')
          : estimate.count === 0
            ? t('estimator.tooSmall')
            : t(estimate.kind === 'clip' ? 'estimator.clips' : 'estimator.posts', { count: shown })}
      </Typography>
    </div>
  )
}
