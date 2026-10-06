import type { ComponentType, ReactNode } from 'react'
import { clsx } from 'clsx'
import { Check, Crown, Gift, Leaf, Rocket, Sparkles, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { planLabel, type PlanName, type PlanOffer } from '@/entities/plan'
import type { BillingTerm } from '@/entities/subscription'
import { PROMO_RISE_STAGGER_MS } from '../config'
import { Badge, PromoFrame, PromoText, Typography } from '@/shared/ui'

/** Each rung's glyph. Presentation only — the ladder's order and figures are the caller's, and
 *  the icon is how a card is told apart at a glance before its name is read. The operator tier
 *  is never on offer, but the type includes it so a future rung cannot arrive without one. */
const TIER_ICON: Record<PlanName, ComponentType<{ className?: string }>> = {
  free: Leaf,
  light: Gift,
  basic: Zap,
  pro: Sparkles,
  max: Rocket,
  master: Crown,
}

/** Shared comparison cards. The caller owns the figures and billing actions; production
 *  estimates and shared conditions live outside the cards (QUOTA-28, QUOTA-41). */
export function PlanLadder({
  offers,
  currentPlan,
  action,
  term,
  headingLevel = 'h2',
  className,
}: {
  offers: readonly PlanOffer[]
  /** The rung the reader is on, which carries the one state label it may have. */
  currentPlan?: PlanName
  /** Every card's one button, in the same slot under the price. Omitted on the public About
   *  ladder, which sells nothing (MKT-5). */
  action?: (offer: PlanOffer) => ReactNode
  /** Omitted on the public About ladder, which presents both list prices. */
  term?: BillingTerm
  /** The outline level the tier names take: `h2` on `/plans`, whose title is the page's `h1`;
   *  `h3` under a section title elsewhere. */
  headingLevel?: 'h2' | 'h3'
  className?: string
}) {
  return (
    <ul className={clsx('grid min-w-0 gap-3 md:grid-cols-2 xl:grid-cols-5 xl:gap-4', className)}>
      {offers.map((offer, index) => (
        <li key={offer.plan} className="min-w-0">
          <PlanCard
            offer={offer}
            index={index}
            current={offer.plan === currentPlan}
            action={action ? action(offer) : undefined}
            hasAction={action !== undefined}
            term={term}
            headingLevel={headingLevel}
          />
        </li>
      ))}
    </ul>
  )
}

/** Tier, price, one button, four benefits — in that reading order and nothing else
 *  (QUOTA-28, THEME-41). */
function PlanCard({
  offer,
  index,
  current,
  action,
  hasAction,
  term,
  headingLevel,
}: {
  offer: PlanOffer
  /** The rung's position, which sets how long after the first it rises into place. */
  index: number
  current: boolean
  action: ReactNode
  /** The slot is reserved on `/plans` even when a card's button is missing, so the five
   *  buttons sit on one line on the desk grid. */
  hasAction: boolean
  term: BillingTerm | undefined
  headingLevel: 'h2' | 'h3'
}) {
  const { t } = useTranslation('plans')
  const Icon = TIER_ICON[offer.plan ?? 'free']
  const priced = offer.monthlyKrw > 0
  const annual = term === 'annual'

  return (
    <PromoFrame
      density="compact"
      marked={offer.recommended}
      // The desk has room for the recommendation to stand a step taller than its neighbours;
      // a phone stack does not, and the halo already says it there. `z-10` lets the lifted card
      // and its halo overlap the rungs beside it instead of being cut by their edges.
      className={clsx('animate-rise', offer.recommended && 'xl:z-10 xl:scale-105')}
      style={{ animationDelay: `${index * PROMO_RISE_STAGGER_MS}ms` }}
    >
      <div className="flex h-full min-w-0 flex-col">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Icon aria-hidden="true" className="text-badge-accent-fg size-5 shrink-0" />
          <Typography variant="title" as={headingLevel}>
            {planLabel(offer.plan)}
          </Typography>
          {current ? (
            <Badge tone="accent">{t('compare.current')}</Badge>
          ) : (
            offer.recommended && <Badge tone="accent">{t('compare.recommended')}</Badge>
          )}
        </div>

        <div className="mt-3 min-h-16">
          <p className="flex flex-wrap items-baseline gap-x-1">
            <PromoText variant="hero" as="span" className="tabular-nums">
              {priced
                ? t('compare.priceKrw', { price: annual ? offer.annualKrw : offer.monthlyKrw })
                : t('compare.priceFree')}
            </PromoText>
            {priced && (
              <Typography variant="meta" as="span" className="text-content-secondary">
                {t(annual ? 'compare.perYear' : 'compare.perMonth')}
              </Typography>
            )}
          </p>
          {priced && annual && (
            <Typography variant="meta" className="text-content-secondary mt-1">
              {t('compare.monthlyEquivalent', { price: Math.round(offer.annualKrw / 12) })}
            </Typography>
          )}
          {/* About has no period selector, so both list prices and the saving are its own. */}
          {priced && term === undefined && (
            <div className="text-content-secondary mt-1 grid gap-0.5">
              <Typography variant="meta">
                {t('compare.annual', { price: offer.annualKrw })}
              </Typography>
              <Typography variant="meta">{t('compare.annualSaving')}</Typography>
            </div>
          )}
        </div>

        {hasAction && <div className="mt-4 min-h-11">{action}</div>}

        <ul className="border-divider mt-4 grid gap-2 border-t pt-4">
          {(priced
            ? [
                t('compare.dailyCredits', { credits: offer.dailyCredits }),
                t('compare.monthlyBonus', { credits: offer.monthlyBonus }),
                offer.modelCeiling === 'none'
                  ? t('compare.freeModels')
                  : t('compare.models', { level: t(`estimator.combos.${offer.modelCeiling}`) }),
                ...(offer.monthlyServerExports > 0
                  ? [t('compare.exports', { count: offer.monthlyServerExports })]
                  : []),
              ]
            : [t('compare.freeModels'), t('compare.freeLimits')]
          ).map((benefit) => (
            <li key={benefit} className="flex min-w-0 items-start gap-2">
              <Check aria-hidden="true" className="text-badge-accent-fg mt-0.5 size-4 shrink-0" />
              <Typography variant="body" as="span" className="text-content-primary min-w-0">
                {benefit}
              </Typography>
            </li>
          ))}
        </ul>
      </div>
    </PromoFrame>
  )
}
