import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  postsPerGrant,
  PLANS,
  useMyPlan,
  planLabel,
  type EstimatorCombo,
  type EstimatorComboName,
  type PlanOffer,
} from '@/entities/plan'
import { billablePlan, useMyBilling, type BillingTerm } from '@/entities/subscription'
import { ScheduledChangeButton } from '@/features/manage-subscription'
import {
  Badge,
  Notice,
  PromoFrame,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { firstCombo, useEstimateInput, type EstimateInput } from '../model/estimate-input'
import { PlanEstimator } from './PlanEstimator'

/** The plan comparison and the place a subscription starts (QUOTA-28). Composition
 *  only: it reads the ladder the server publishes and renders it.
 *
 *  Every figure — the grant, the price, which rung is recommended, which is current — comes
 *  from GetMyPlan. A price kept here would eventually disagree with
 *  the grant beside it, the grant is the half that is actually enforced, and a recommendation
 *  the client invented would be emphasis the ladder never asked for.
 *
 *  Nothing on this screen charges anyone: a paid rung only hands the selection to BILLING's
 *  checkout, which owns the term, quote, payment method and committing action. */
export function PlansPage() {
  const { t } = useTranslation(['plans', 'common'])
  const { myPlan, isPending, isError } = useMyPlan()
  const { myBilling } = useMyBilling()
  const [input, setInput] = useEstimateInput()
  const [chosen, setChosen] = useState<EstimatorComboName | undefined>(undefined)

  const combos = myPlan?.estimatorCombos ?? []
  // The reader's choice while it is still assigned, otherwise the first tier the server
  // published: a combo the operator has since replaced must not leave the page with a
  // selection nothing can price.
  const combo = combos.some((assigned) => assigned.combo === chosen) ? chosen : firstCombo(combos)
  const rates = combos.find((assigned) => assigned.combo === combo)

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
          <span className="grid gap-2">
            <span>
              <Typography variant="label" as="span">
                {t('compare.blockedHeading', { ns: 'plans' })}
              </Typography>{' '}
              {t('compare.blockedBody', { ns: 'plans' })}
            </span>
            <Link
              to="/billing"
              className={typographyStyles({
                variant: 'label',
                className: 'text-link-fg hover:text-link-fg-hover w-fit underline',
              })}
            >
              {t('compare.buyCredits', { ns: 'plans' })}
            </Link>
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
          <PlanEstimator
            combos={combos}
            combo={combo}
            input={input}
            onComboChange={setChosen}
            onInputChange={setInput}
          />
          {/* Stacked on a phone and side by side upward: four rungs at `md:` leave ~180px a
              card, which wraps every figure onto its own ragged line, so the shape changes
              at `md:` in two columns and only reaches four on the desk (THEME-14). */}
          <ul className="mt-4 grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {myPlan.offers.map((offer) => (
              <li key={offer.plan}>
                <PlanCard
                  offer={offer}
                  current={offer.plan === myPlan.plan}
                  actions={myPlan.plan !== 'master'}
                  rates={rates}
                  input={input}
                  subscribedPlan={
                    myBilling?.subscription?.status === 'active'
                      ? myBilling.subscription.plan
                      : undefined
                  }
                  subscribedTerm={myBilling?.subscription?.term}
                />
              </li>
            ))}
          </ul>
          {rates && (
            /* The assumption behind every count, once under the list rather than four times
               inside it: repeated in every card it would read as fine print. */
            <Typography variant="meta" className="text-content-tertiary mt-4 block">
              {t('estimator.caveat', { ns: 'plans' })}
            </Typography>
          )}
        </>
      )}
    </main>
  )
}

/** One rung. A card here is the case §1.4 allows: an item in a grid of peers, whose contents
 *  read as one unit and apart from the rung above and below it.
 *
 *  Every rung wears the promotional stroke and the recommended one wears it wider, which is
 *  the exception THEME-37 opens for a surface whose job is to be chosen from. The badge, not
 *  the glow, is what says which rung is recommended, and the flag comes from the server so a
 *  second marked card is impossible by construction rather than by review. It gets no filled
 *  CTA — one CTA per view. A paid rung links to checkout; free and the operator account have
 *  no commercial action. */
function PlanCard({
  offer,
  current,
  actions,
  rates,
  input,
  subscribedPlan,
  subscribedTerm,
}: {
  offer: PlanOffer
  current: boolean
  actions: boolean
  rates: EstimatorCombo | undefined
  input: EstimateInput
  subscribedPlan: PlanOffer['plan']
  subscribedTerm: BillingTerm | undefined
}) {
  const { t } = useTranslation('plans')
  // The whole of this card's arithmetic: the server owns the charge formula and published the
  // rates (QUOTA-40). `master` is not on offer, so no rung here is ever unlimited.
  const posts = rates ? postsPerGrant(offer.monthlyCredits, rates, input) : undefined

  return (
    <PromoFrame marked={offer.recommended}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <Typography variant="title" as="h2">
          {planLabel(offer.plan)}
        </Typography>
        {current && <Badge tone="accent">{t('compare.current')}</Badge>}
        {!current && offer.recommended && <Badge tone="accent">{t('compare.recommended')}</Badge>}
      </div>
      {/* The card's own headline figure once a case is set: `fieldTitle` is the role that
          carries weight without competing with the tier's own heading (THEME-19). */}
      {posts !== undefined && (
        <Typography variant="fieldTitle" as="p" className="mt-2">
          {posts > 0 ? t('estimator.posts', { count: posts }) : t('estimator.tooSmall')}
        </Typography>
      )}
      <Typography variant="body" className="text-content-secondary mt-2 block">
        {t('compare.monthlyCredits', { credits: offer.monthlyCredits })}
      </Typography>
      <Typography variant="body" className="mt-1 block">
        {offer.priceUsdCents > 0
          ? t('compare.price', { usd: (offer.priceUsdCents / 100).toFixed(0) })
          : t('compare.priceFree')}
      </Typography>
      {!current && actions && billablePlan(offer.plan) && (
        <div className="mt-3">
          {subscribedPlan &&
          subscribedTerm &&
          PLANS.indexOf(offer.plan) < PLANS.indexOf(subscribedPlan) ? (
            <ScheduledChangeButton
              plan={offer.plan}
              term={subscribedTerm}
              label={t('compare.nextBilling')}
            />
          ) : (
            <Link
              to="/billing/checkout"
              search={{ tier: offer.plan }}
              className={buttonStyles({ variant: 'secondary' })}
            >
              {subscribedPlan ? t('compare.upgrade') : t('compare.select')}
            </Link>
          )}
        </div>
      )}
      {!current && actions && offer.plan === 'free' && subscribedPlan && (
        <Link to="/billing" className={buttonStyles({ variant: 'secondary', className: 'mt-3' })}>
          {t('compare.cancelFromBilling')}
        </Link>
      )}
    </PromoFrame>
  )
}
