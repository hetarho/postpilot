import { useTranslation } from 'react-i18next'
import { useMyPlan, planLabel, type PlanOffer } from '@/entities/plan'
import { Badge, Button, Notice, Typography } from '@/shared/ui'

/** The plan comparison (change 19). Composition only: it reads the ladder the server
 *  publishes and renders it.
 *
 *  Every figure — the grant, the price, which rung is current — comes from GetMyPlan. A
 *  price kept here would eventually disagree with the grant beside it, and the grant is the
 *  half that is actually enforced.
 *
 *  This screen ends exactly where a checkout would begin. Nothing on it charges anyone
 *  (PRD §9): the action names the tier and then says plans are operator-assigned, which is
 *  the true state of the product rather than a disabled button with no explanation. */
export function PlansPage() {
  const { t } = useTranslation(['plans', 'common'])
  const { myPlan, isPending, isError } = useMyPlan()

  const empty = myPlan !== undefined && !myPlan.balance.unlimited && myPlan.balance.credits <= 0

  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
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
          <ul className="mt-8 grid gap-4">
            {myPlan.offers.map((offer) => (
              <li key={offer.plan}>
                <PlanCard offer={offer} current={offer.plan === myPlan.plan} />
              </li>
            ))}
          </ul>
          <Typography variant="meta" className="text-content-tertiary mt-6 block">
            {t('compare.notPurchasable', { ns: 'plans' })}
          </Typography>
        </>
      )}
    </main>
  )
}

/** One rung. A card here is the case §1.4 allows: an item in a grid of peers, whose contents
 *  read as one unit and apart from the rung above and below it. */
function PlanCard({ offer, current }: { offer: PlanOffer; current: boolean }) {
  const { t } = useTranslation('plans')

  return (
    <div className="bg-surface-raised rounded-lg p-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <Typography variant="title" as="h2">
          {planLabel(offer.plan)}
        </Typography>
        {current && <Badge tone="accent">{t('compare.current')}</Badge>}
      </div>
      <Typography variant="body" className="text-content-secondary mt-2 block">
        {t('compare.monthlyCredits', { credits: offer.monthlyCredits })}
      </Typography>
      <Typography variant="body" className="mt-1 block">
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
