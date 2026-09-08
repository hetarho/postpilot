import type { ComponentType, ReactNode } from 'react'
import { clsx } from 'clsx'
import { Crown, Leaf, Rocket, Sparkles, Zap } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { planLabel, type PlanName, type PlanOffer } from '@/entities/plan'
import { PROMO_COUNT_UP_MS, PROMO_RISE_STAGGER_MS } from '@/shared/config'
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
  estimate,
  action,
  headingLevel = 'h2',
  className,
}: {
  offers: readonly PlanOffer[]
  /** The rung the reader is on, which is named and offered no action. */
  currentPlan?: PlanName
  /** How many posts of the reader's case a rung's grant buys; `undefined` shows no estimate. */
  estimate?: (offer: PlanOffer) => number | undefined
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
            posts={estimate?.(offer)}
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
  posts,
  action,
  headingLevel,
}: {
  offer: PlanOffer
  /** The rung's position, which sets how long after the first it rises into place. */
  index: number
  current: boolean
  posts: number | undefined
  action: ReactNode
  headingLevel: 'h2' | 'h3'
}) {
  const { t } = useTranslation('plans')
  const Icon = TIER_ICON[offer.plan ?? 'free']
  const priced = offer.priceUsdCents > 0

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
        <div className="flex items-start justify-between gap-2">
          {/* The glyph sits on the accent's quiet plate — the same pair the status chip beside
              it wears — so the card opens with one violet mark and nothing competing for it. */}
          <span
            aria-hidden="true"
            className="bg-badge-accent-bg text-badge-accent-fg inline-flex size-10 shrink-0 items-center justify-center rounded-full"
          >
            <Icon className="size-5" />
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
        {posts !== undefined && <PostEstimate posts={posts} />}
        <Typography variant="body" className="text-content-secondary mt-3 block">
          {t('compare.monthlyCredits', { credits: offer.monthlyCredits })}
        </Typography>
        {action && <div className="mt-auto pt-5">{action}</div>}
      </div>
    </PromoFrame>
  )
}

/** The card's own headline once a case is set — what THIS grant buys of the reader's post — on
 *  the accent plate so the four figures line up as the row a reader actually compares. The count
 *  climbs to its new value as a slider moves rather than flickering through replacements. */
function PostEstimate({ posts }: { posts: number }) {
  const { t } = useTranslation('plans')
  const shown = useCountUp(posts, PROMO_COUNT_UP_MS)
  return (
    <Typography
      variant="fieldTitle"
      as="p"
      className="bg-badge-accent-bg text-badge-accent-fg mt-4 rounded-md px-3 py-2 tabular-nums"
    >
      {shown > 0 ? t('estimator.posts', { count: shown }) : t('estimator.tooSmall')}
    </Typography>
  )
}
