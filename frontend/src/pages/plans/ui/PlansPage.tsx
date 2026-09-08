import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  postsPerGrant,
  PLANS,
  useMyPlan,
  type EstimatorComboName,
  type PlanOffer,
} from '@/entities/plan'
import { billablePlan, useMyBilling } from '@/entities/subscription'
import { ScheduledChangeButton } from '@/features/manage-subscription'
import {
  Notice,
  PromoStage,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { PlanLadder } from '@/widgets/plan-ladder'
import { firstCombo, useEstimateInput } from '../model/estimate-input'
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
 *  checkout, which owns the term, quote, payment method and committing action. The recommended
 *  rung's subscribe action is the view's ONE filled CTA (THEME-18): the product is pointing at
 *  it, so the button may too. */
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
  const subscribedPlan =
    myBilling?.subscription?.status === 'active' ? myBilling.subscription.plan : undefined
  const subscribedTerm = myBilling?.subscription?.term

  /** A rung's one action. A paid rung links to checkout — as an upgrade when a subscription
   *  exists, as a scheduled change when it is lower than the one paid for; the free rung under
   *  a subscription points at Billing, which owns cancellation; the current rung and the
   *  operator account have no commercial action. */
  const action = (offer: PlanOffer) => {
    if (!myPlan || offer.plan === myPlan.plan || myPlan.plan === 'master') return null
    if (billablePlan(offer.plan)) {
      if (
        offer.plan !== undefined &&
        subscribedPlan !== undefined &&
        subscribedTerm !== undefined &&
        PLANS.indexOf(offer.plan) < PLANS.indexOf(subscribedPlan)
      ) {
        return (
          <ScheduledChangeButton
            plan={offer.plan}
            term={subscribedTerm}
            label={t('compare.nextBilling', { ns: 'plans' })}
          />
        )
      }
      return (
        <Link
          to="/billing/checkout"
          search={{ tier: offer.plan }}
          className={buttonStyles({
            variant: offer.recommended ? 'cta' : 'secondary',
            className: 'w-full',
          })}
        >
          {subscribedPlan
            ? t('compare.upgrade', { ns: 'plans' })
            : t('compare.select', { ns: 'plans' })}
        </Link>
      )
    }
    if (offer.plan === 'free' && subscribedPlan) {
      return (
        <Link to="/billing" className={buttonStyles({ variant: 'secondary', className: 'w-full' })}>
          {t('compare.cancelFromBilling', { ns: 'plans' })}
        </Link>
      )
    }
    return null
  }

  return (
    <main className={pageStyles({ width: 'board' })}>
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
        // The estimator and the ladder it prices share one stage: they are one promotional
        // surface (THEME-37), and a plain estimator above a lit ladder would read as two screens.
        <PromoStage className="mt-8">
          <PlanEstimator
            combos={combos}
            combo={combo}
            input={input}
            onComboChange={setChosen}
            onInputChange={setInput}
          />
          <PlanLadder
            className="mt-6"
            offers={myPlan.offers}
            currentPlan={myPlan.plan}
            // The whole of this page's arithmetic: the server owns the charge formula and
            // published the rates (QUOTA-40). `master` is not on offer, so no rung is unlimited.
            estimate={
              rates ? (offer) => postsPerGrant(offer.monthlyCredits, rates, input) : undefined
            }
            action={action}
          />
          {rates && (
            /* The assumption behind every count, once under the list rather than four times
               inside it: repeated in every card it would read as fine print. */
            <Typography variant="meta" className="text-content-tertiary mt-4 block">
              {t('estimator.caveat', { ns: 'plans' })}
            </Typography>
          )}
        </PromoStage>
      )}
    </main>
  )
}
