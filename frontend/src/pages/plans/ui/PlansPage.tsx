import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { Check, SlidersHorizontal, Sparkles, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  clipsPerGrant,
  ESTIMATOR_COMBOS,
  postsPerGrant,
  PLANS,
  useMyPlan,
  type PlanOffer,
} from '@/entities/plan'
import { billablePlan, useMyBilling } from '@/entities/subscription'
import { ScheduledChangeButton } from '@/features/manage-subscription'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  PromoText,
  Sheet,
  SegmentedControl,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { PlanLadder } from '@/widgets/plan-ladder'
import { useClipEstimateInput, useEstimateInput, type EstimateKind } from '../model/estimate-input'
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
  const [estimatorOpen, setEstimatorOpen] = useState(false)
  const [kind, setKind] = useState<EstimateKind>('blog')
  const [clipInput, setClipInput] = useClipEstimateInput()
  const combos = myPlan?.estimatorCombos ?? []

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
    <main
      className={pageStyles({
        width: 'board',
        className: 'relative flex-1 pb-28 sm:pt-10 sm:pb-28',
      })}
    >
      <SegmentedControl
        ariaLabel={t('estimator.basis', { ns: 'plans' })}
        className="mx-auto mb-8 max-w-md"
        value={kind}
        options={[
          { value: 'blog', label: t('estimator.blogBasis', { ns: 'plans' }) },
          { value: 'clip', label: t('estimator.clipBasis', { ns: 'plans' }) },
        ]}
        onChange={setKind}
      />
      <header className="animate-rise mx-auto flex max-w-3xl flex-col items-center pb-8 text-center sm:pb-12">
        <Badge tone="accent">
          <Sparkles aria-hidden="true" className="mr-1.5 inline size-3.5" />
          {t('compare.title', { ns: 'plans' })}
        </Badge>
        <Typography variant="promoDisplay" className="mt-6 text-balance">
          {t('compare.headline', { ns: 'plans' })}
          <PromoText variant="promoDisplay" as="span" className="mt-1 block">
            {t('compare.headlineAccent', { ns: 'plans' })}
          </PromoText>
        </Typography>
        <Typography
          variant="body"
          className="text-content-secondary max-w-measure mt-5 text-balance"
        >
          {t('compare.description', { ns: 'plans' })}
        </Typography>
        <div className="mt-5 flex flex-wrap justify-center gap-x-5 gap-y-2">
          {(['allModels', 'monthlyRefill', 'editAllowance'] as const).map((benefit) => (
            <Typography key={benefit} variant="label" className="inline-flex items-center gap-1.5">
              <Check aria-hidden="true" className="text-badge-accent-fg size-4 shrink-0" />
              {t(`compare.${benefit}`, { ns: 'plans' })}
            </Typography>
          ))}
        </div>
      </header>

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
        <section aria-label={t('compare.title', { ns: 'plans' })}>
          <div
            className="mx-auto mb-8 flex max-w-3xl flex-col items-center gap-2 text-center"
            aria-live="polite"
          >
            <Typography variant="label" className="text-badge-accent-fg">
              {t(kind === 'blog' ? 'estimator.blogCondition' : 'estimator.clipCondition', {
                ns: 'plans',
              })}
            </Typography>
            <Typography variant="fieldTitle" className="text-balance tabular-nums">
              {kind === 'blog'
                ? t('estimator.summary', { ns: 'plans', ...input })
                : t('estimator.clipSummary', { ns: 'plans', ...clipInput })}
            </Typography>
            {kind === 'clip' && myPlan.clipSourceSeconds > 0 && (
              <Typography variant="meta" className="text-content-secondary">
                {t('estimator.sourceAssumption', {
                  ns: 'plans',
                  seconds: myPlan.clipSourceSeconds,
                })}
              </Typography>
            )}
          </div>
          <Typography variant="body" className="text-content-secondary mt-6 mb-6 text-center">
            {t('benefits.baseline', { ns: 'plans' })}
          </Typography>
          <PlanLadder
            className="mt-2 md:mt-10"
            offers={myPlan.offers}
            currentPlan={myPlan.plan}
            estimates={(offer) =>
              ESTIMATOR_COMBOS.map((level) => {
                const rates = combos.find((assigned) => assigned.combo === level)
                const count =
                  kind === 'blog'
                    ? rates && postsPerGrant(offer.monthlyCredits, rates, input)
                    : rates?.clipRates &&
                      clipsPerGrant(offer.monthlyCredits, rates.clipRates, clipInput)
                return { level, kind, count }
              })
            }
            action={action}
          />
          <Typography
            variant="meta"
            className="text-content-secondary max-w-measure mx-auto mt-8 block text-center"
          >
            {t(kind === 'blog' ? 'estimator.caveat' : 'estimator.clipCaveat', { ns: 'plans' })}
          </Typography>
          <ActionBar dock="list" className="fixed right-4 sm:right-6 lg:right-8">
            <Button
              variant="secondary"
              className="gap-2 rounded-full px-5"
              aria-haspopup="dialog"
              aria-expanded={estimatorOpen}
              onClick={() => setEstimatorOpen(true)}
            >
              <SlidersHorizontal aria-hidden="true" className="size-4 shrink-0" />
              {t('estimator.title', { ns: 'plans' })}
            </Button>
          </ActionBar>
          <Typography variant="body" className="text-content-secondary mt-5 text-center">
            {t('compare.closing', { ns: 'plans' })}
          </Typography>
          <Sheet
            open={estimatorOpen}
            onClose={() => setEstimatorOpen(false)}
            labelledBy="plan-estimator-title"
            header={
              <div className="mb-4 flex items-center justify-between gap-3">
                <Typography variant="title" id="plan-estimator-title">
                  {t(kind === 'blog' ? 'estimator.blogCondition' : 'estimator.clipCondition', {
                    ns: 'plans',
                  })}
                </Typography>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t('action.close', { ns: 'common' })}
                  onClick={() => setEstimatorOpen(false)}
                >
                  <X aria-hidden="true" className="size-5" />
                </Button>
              </div>
            }
            footer={
              <Button
                variant="secondary"
                className="mt-5 w-full"
                onClick={() => setEstimatorOpen(false)}
              >
                {t('estimator.viewPlans', { ns: 'plans' })}
              </Button>
            }
          >
            <PlanEstimator
              kind={kind}
              input={input}
              clipInput={clipInput}
              sourceSeconds={myPlan.clipSourceSeconds}
              onInputChange={setInput}
              onClipInputChange={setClipInput}
            />
            <Typography variant="meta" as="p" className="mt-4">
              {t('estimator.allowance', { ns: 'plans' })}
            </Typography>
          </Sheet>
        </section>
      )}
    </main>
  )
}
