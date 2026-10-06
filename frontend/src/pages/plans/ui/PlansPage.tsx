import { Link } from '@tanstack/react-router'
import { useId, useState, type ReactNode } from 'react'
import { ChevronDown, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import {
  clipsPerGrant,
  illustrativeMonthlyCredits,
  postsPerFigure,
  PLANS,
  useMyPlan,
  type EstimatorCombo,
  type MyPlan,
  type PlanOffer,
} from '@/entities/plan'
import { billablePlan, useMyBilling, type BillingTerm } from '@/entities/subscription'
import { ScheduledChangeButton } from '@/features/manage-subscription'
import {
  Badge,
  Button,
  Notice,
  PromoText,
  SegmentedControl,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import {
  PlanComparison,
  PlanLadder,
  type PlanComparisonFigure,
  type PlanComparisonRow,
} from '@/widgets/plan-ladder'
import { useClipEstimateInput, type ClipEstimateInput } from '../model/estimate-input'
import { PlanEstimator } from './PlanEstimator'

/** A card's non-CTA button. The secondary plane is the card's own raised surface, so on a plan
 *  card it read as bare text; the accent's subtle plane keeps it a visible control in both themes
 *  without competing with pro's one filled CTA (THEME-37 lets promotional surfaces use it). */
const CARD_BUTTON =
  'w-full bg-badge-accent-bg text-badge-accent-fg hover:bg-badge-accent-bg hover:brightness-95 active:bg-badge-accent-bg active:brightness-90'

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
 *  it, so the button may too. Master reads the page as a customer does, with every button it
 *  may not press disabled (QUOTA-68). */
export function PlansPage() {
  const { t } = useTranslation(['plans', 'common'])
  const { myPlan, isPending, isError } = useMyPlan()
  const { myBilling } = useMyBilling()
  const [term, setTerm] = useState<BillingTerm>('monthly')
  const [clipInput, setClipInput] = useClipEstimateInput()

  const empty = myPlan !== undefined && !myPlan.balance.unlimited && myPlan.balance.credits <= 0
  const subscribedPlan =
    myBilling?.subscription?.status === 'active' ? myBilling.subscription.plan : undefined
  const subscribedTerm = myBilling?.subscription?.term

  /** A rung's one button, chosen by billing state (QUOTA-28). A state with nothing to press
   *  still shows a disabled button in the same slot, so the five cards read alike. */
  const action = (offer: PlanOffer): ReactNode => {
    if (!myPlan || offer.plan === undefined) return null
    const disabled = (label: string) => (
      <Button variant="secondary" disabled className={CARD_BUTTON}>
        {label}
      </Button>
    )
    // The operator is never charged (BILL-20), so it sees a customer's first-purchase buttons,
    // none of which it can press.
    if (myPlan.plan === 'master') {
      return disabled(
        billablePlan(offer.plan)
          ? t('compare.select', { ns: 'plans' })
          : t('compare.freePlan', { ns: 'plans' }),
      )
    }
    if (offer.plan === myPlan.plan) {
      return billablePlan(offer.plan) ? (
        <Link
          to="/billing"
          className={buttonStyles({ variant: 'secondary', className: CARD_BUTTON })}
        >
          {t('compare.manage', { ns: 'plans' })}
        </Link>
      ) : (
        disabled(t('compare.inUse', { ns: 'plans' }))
      )
    }
    if (billablePlan(offer.plan)) {
      if (
        subscribedPlan !== undefined &&
        subscribedTerm !== undefined &&
        PLANS.indexOf(offer.plan) < PLANS.indexOf(subscribedPlan)
      ) {
        return (
          <ScheduledChangeButton
            plan={offer.plan}
            term={subscribedTerm}
            label={t('compare.nextBilling', { ns: 'plans' })}
            className={CARD_BUTTON}
          />
        )
      }
      return (
        <Link
          to="/billing/checkout"
          search={{ tier: offer.plan, term: subscribedTerm ?? term }}
          className={buttonStyles(
            offer.recommended
              ? { variant: 'cta', className: 'w-full' }
              : { variant: 'secondary', className: CARD_BUTTON },
          )}
        >
          {subscribedPlan
            ? t('compare.upgrade', { ns: 'plans' })
            : t('compare.select', { ns: 'plans' })}
        </Link>
      )
    }
    if (subscribedPlan) {
      return (
        <Link
          to="/billing"
          className={buttonStyles({ variant: 'secondary', className: CARD_BUTTON })}
        >
          {t('compare.cancelFromBilling', { ns: 'plans' })}
        </Link>
      )
    }
    return disabled(t('compare.freePlan', { ns: 'plans' }))
  }

  return (
    <main
      className={pageStyles({
        width: 'board',
        className: 'relative flex-1 pb-16 sm:pt-10 sm:pb-20',
      })}
    >
      <header className="animate-rise mx-auto flex max-w-3xl flex-col items-center pb-4 text-center sm:pb-8">
        <Badge tone="accent">
          <Sparkles aria-hidden="true" className="mr-1.5 inline size-3.5" />
          {t('compare.title', { ns: 'plans' })}
        </Badge>
        <Typography variant="promoDisplay" className="mt-3 text-balance sm:mt-6">
          {t('compare.headline', { ns: 'plans' })}
          <PromoText variant="promoDisplay" as="span" className="mt-1 block">
            {t('compare.headlineAccent', { ns: 'plans' })}
          </PromoText>
        </Typography>
        <Typography
          variant="body"
          className="text-content-secondary max-w-measure mt-3 text-balance sm:mt-5"
        >
          {t('compare.description', { ns: 'plans' })}
        </Typography>
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
        <>
          <section aria-label={t('compare.title', { ns: 'plans' })}>
            <div className="mx-auto mb-5 max-w-md">
              <SegmentedControl
                ariaLabel={t('compare.period', { ns: 'plans' })}
                value={term}
                options={
                  [
                    { value: 'monthly', label: t('compare.monthly', { ns: 'plans' }) },
                    { value: 'annual', label: t('compare.yearly', { ns: 'plans' }) },
                  ] as const
                }
                onChange={setTerm}
              />
              {/* The ten-for-twelve saving is said once, here, never on a card (QUOTA-28). */}
              {term === 'annual' && (
                <Typography
                  variant="meta"
                  className="text-content-secondary mt-2 block text-center"
                >
                  {t('compare.termSaving', { ns: 'plans' })}
                </Typography>
              )}
              {subscribedPlan && (
                <Typography
                  variant="meta"
                  className="text-content-secondary mt-2 block text-center"
                >
                  {t('compare.existingTermNote', { ns: 'plans' })}{' '}
                  <Link to="/billing" className="text-link-fg underline">
                    {t('compare.billingSettings', { ns: 'plans' })}
                  </Link>
                </Typography>
              )}
            </div>
            <PlanLadder
              className="mt-2 md:mt-6"
              offers={myPlan.offers}
              currentPlan={myPlan.plan}
              term={term}
              action={action}
            />
            <Typography variant="meta" className="text-content-secondary mt-6 block text-center">
              {t('compare.clipCap', { ns: 'plans' })}
            </Typography>
            <Typography variant="meta" className="text-content-secondary mt-1 block text-center">
              {t('compare.browserRendering', { ns: 'plans' })}
            </Typography>
          </section>

          <ProductionComparison
            myPlan={myPlan}
            clipInput={clipInput}
            onClipInputChange={setClipInput}
          />

          <Typography variant="body" className="text-content-secondary mt-10 text-center">
            {t('compare.closing', { ns: 'plans' })}
          </Typography>
        </>
      )}
    </main>
  )
}

/** Posts and AI clips a month for every tier, together below the cards (QUOTA-36, QUOTA-41).
 *  Each paid tier is priced on its own highest level only; a level whose pair is missing,
 *  unpriced or incompatible says so instead of borrowing another level's count (QUOTA-56). */
function ProductionComparison({
  myPlan,
  clipInput,
  onClipInputChange,
}: {
  myPlan: MyPlan
  clipInput: ClipEstimateInput
  onClipInputChange: (input: ClipEstimateInput) => void
}) {
  const { t } = useTranslation('plans')
  const titleId = useId()
  const conditionsId = useId()
  const [conditionsOpen, setConditionsOpen] = useState(false)
  const rows = myPlan.offers.flatMap((offer): PlanComparisonRow[] => {
    if (offer.plan === undefined) return []
    if (offer.modelCeiling === 'none') return [{ plan: offer.plan }]
    const combo = myPlan.estimatorCombos.find((assigned) => assigned.combo === offer.modelCeiling)
    return [
      {
        plan: offer.plan,
        level: offer.modelCeiling,
        ...comparisonFigures(offer, combo, myPlan.fxUnavailable, clipInput),
        ...(combo?.postCredits && !myPlan.fxUnavailable && { perPost: combo.postCredits }),
      },
    ]
  })

  return (
    <section aria-labelledby={titleId} className="mx-auto mt-12 grid max-w-3xl gap-4">
      <div className="text-center">
        <Typography variant="title" as="h2" id={titleId}>
          {t('comparison.title')}
        </Typography>
        <Typography variant="body" className="text-content-secondary mt-2 text-balance">
          {t('benefits.baseline')}
        </Typography>
      </div>
      {myPlan.fxUnavailable && (
        <Notice tone="info" role="status">
          {t('estimator.fxUnavailable')}
        </Notice>
      )}
      <div>
        <Button
          variant="secondary"
          className="w-full justify-between gap-3 text-left"
          aria-expanded={conditionsOpen}
          aria-controls={conditionsId}
          onClick={() => setConditionsOpen((open) => !open)}
        >
          <span className="min-w-0">
            {t('comparison.conditions')}
            {' · '}
            <span className="tabular-nums">
              {t('estimator.clipSummary', {
                sources: clipInput.sources,
                seconds: clipInput.seconds,
              })}
            </span>
          </span>
          <ChevronDown
            aria-hidden="true"
            className={conditionsOpen ? 'size-4 shrink-0 rotate-180' : 'size-4 shrink-0'}
          />
        </Button>
        {conditionsOpen && (
          <div id={conditionsId} className="bg-surface-raised mt-2 rounded-md p-4">
            <PlanEstimator
              clipInput={clipInput}
              sourceSeconds={myPlan.clipSourceSeconds}
              onClipInputChange={onClipInputChange}
            />
          </div>
        )}
      </div>
      <PlanComparison rows={rows} />
      <div className="grid gap-2">
        <Typography variant="meta" className="text-content-secondary">
          {t('estimator.caveat')}
        </Typography>
        <Typography variant="meta" className="text-content-secondary">
          {t('estimator.clipCaveat')}
        </Typography>
      </div>
    </section>
  )
}

function comparisonFigures(
  offer: PlanOffer,
  combo: EstimatorCombo | undefined,
  fxUnavailable: boolean,
  clipInput: ClipEstimateInput,
): { posts: PlanComparisonFigure; clips: PlanComparisonFigure } {
  if (fxUnavailable) return { posts: { unavailable: 'rate' }, clips: { unavailable: 'rate' } }
  const credits = illustrativeMonthlyCredits(offer)
  return {
    posts: combo?.postCredits
      ? { count: postsPerFigure(credits, combo.postCredits.credits) }
      : { unavailable: 'pricing' },
    clips: combo?.clipRates
      ? { count: clipsPerGrant(credits, combo.clipRates, clipInput) }
      : { unavailable: combo ? 'clip' : 'pricing' },
  }
}
