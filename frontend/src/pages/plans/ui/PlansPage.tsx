import { useTranslation } from 'react-i18next'
import { useMyPlan, planLabel, type PlanOffer } from '@/entities/plan'
import { Badge, Button, Notice, Typography, pageStyles } from '@/shared/ui'

/** The plan comparison, and the place a subscription will start (QUOTA-28). Composition
 *  only: it reads the ladder the server publishes and renders it.
 *
 *  Every figure — the grant, the price, how many posts it buys, which rung is recommended,
 *  which is current — comes from GetMyPlan. A price kept here would eventually disagree with
 *  the grant beside it, the grant is the half that is actually enforced, and a recommendation
 *  the client invented would be emphasis the ladder never asked for.
 *
 *  This screen ends exactly where a checkout would begin. Nothing on it charges anyone: the
 *  action names the tier and then says plans are operator-assigned, which is the true state
 *  of the product until BILLING ships rather than a disabled button with no explanation. */
export function PlansPage() {
  const { t } = useTranslation(['plans', 'common'])
  const { myPlan, isPending, isError } = useMyPlan()

  const empty = myPlan !== undefined && !myPlan.balance.unlimited && myPlan.balance.credits <= 0

  return (
    <main className={pageStyles()}>
      <Typography variant="display">{t('compare.title', { ns: 'plans' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('compare.description', { ns: 'plans' })}
      </Typography>

      {/* The one thing a user arriving here from a refusal needs told: what still works. */}
      {empty && (
        <Notice tone="info" role="status" className="mt-6">
          <span>
            <Typography variant="label" as="span">
              {t('compare.blockedHeading', { ns: 'plans' })}
            </Typography>{' '}
            {t('compare.blockedBody', { ns: 'plans' })}
          </span>
        </Notice>
      )}

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          {t('balance.loadFailed', { ns: 'plans' })}
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!isError && !isPending && myPlan && (
        <>
          {/* Stacked on a phone and side by side upward: four rungs at `md:` leave ~180px a
              card, which wraps every figure onto its own ragged line, so the shape changes
              at `md:` in two columns and only reaches four on the desk (THEME-14). */}
          <ul className="mt-8 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {myPlan.offers.map((offer) => (
              <li key={offer.plan}>
                <PlanCard offer={offer} current={offer.plan === myPlan.plan} />
              </li>
            ))}
          </ul>
          {/* The assumption behind every estimate, once under the list rather than four times
              inside it: repeated in every card it would read as fine print, and the list is
              one comparison. */}
          <Typography variant="meta" className="text-content-tertiary mt-4 block">
            {t('compare.estimateCaveat', { ns: 'plans' })}
          </Typography>
          <Typography variant="meta" className="text-content-tertiary mt-6 block">
            {t('compare.notPurchasable', { ns: 'plans' })}
          </Typography>
        </>
      )}
    </main>
  )
}

/** One rung. A card here is the case §1.4 allows: an item in a grid of peers, whose contents
 *  read as one unit and apart from the rung above and below it.
 *
 *  The recommended rung is the one border in the app that is not one of the four structural
 *  exceptions: a promotional surface may mark exactly ONE option with an accent stroke
 *  (THEME-37). The stroke, the shadow and the badge all hang off the single `recommended`
 *  flag the server sends, so a second emphasised card is impossible by construction rather
 *  than by review. It gets no filled CTA — one CTA per view, and every rung's action is
 *  disabled until BILLING ships. */
function PlanCard({ offer, current }: { offer: PlanOffer; current: boolean }) {
  const { t } = useTranslation('plans')

  return (
    <div
      className={
        offer.recommended
          ? 'bg-surface-raised border-stroke-accent rounded-lg border p-4 shadow-md'
          : 'bg-surface-raised rounded-lg p-4'
      }
    >
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <Typography variant="title" as="h2">
          {planLabel(offer.plan)}
        </Typography>
        {current && <Badge tone="accent">{t('compare.current')}</Badge>}
        {!current && offer.recommended && <Badge tone="accent">{t('compare.recommended')}</Badge>}
      </div>
      <Typography variant="body" className="text-content-secondary mt-2 block">
        {t('compare.monthlyCredits', { credits: offer.monthlyCredits })}
      </Typography>
      {/* What the grant buys, in product terms rather than in credits (QUOTA-36). The figure
          is the server's; this page does no arithmetic on it. */}
      <Typography variant="body" className="text-content-secondary mt-1 block">
        {offer.estimatedPosts > 0
          ? t('compare.estimatedPosts', { count: offer.estimatedPosts })
          : t('compare.estimateNone')}
      </Typography>
      <Typography variant="body" className="mt-2 block">
        {offer.priceUsdCents > 0
          ? t('compare.price', { usd: (offer.priceUsdCents / 100).toFixed(0) })
          : t('compare.priceFree')}
      </Typography>
      {!current && (
        <Button variant="secondary" disabled className="mt-3">
          {t('compare.select')}
        </Button>
      )}
    </div>
  )
}
