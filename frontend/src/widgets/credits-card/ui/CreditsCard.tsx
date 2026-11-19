import { useEffect, useId, useRef, useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Zap } from 'lucide-react'
import { planLabel, useMyPlan } from '@/entities/plan'
import { useMyBilling } from '@/entities/subscription'
import { CreditPurchaseSection } from '@/features/purchase-credits'
import { creditsIllustration } from '@/shared/assets'
import {
  Button,
  LoadingState,
  Notice,
  Sheet,
  buttonStyles,
  cardStyles,
  typographyStyles,
} from '@/shared/ui'

/** The credits card 설정 and the billing screen both open on (QUOTA-75): what is left to spend,
 *  the plan it comes from, and ONE 충전. A paid tier's 충전 brings the fixed packs up in a sheet
 *  over the screen it was pressed on, so buying more never leaves it (BILL-14); a free account
 *  cannot buy a pack (BILL-9), so its 충전 is subscribing on `/plans` (→QUOTA-28). Master's 충전
 *  reaches the packs' own disabled state, as a customer's screen would show it (QUOTA-68). */
export function CreditsCard({
  openPacks = false,
  className,
}: {
  /** Brings the packs sheet up once, the first time it turns true — an arrival at
   *  `/billing#billing-charge`. A free account has no packs to arrive at and ignores it. */
  openPacks?: boolean
  className?: string
}) {
  const { t } = useTranslation(['billing', 'plans', 'common'])
  const headingId = useId()
  const { myPlan, isPending } = useMyPlan()
  const [packsOpen, setPacksOpen] = useState(false)
  const chargeToPlans = myPlan?.plan === 'free'
  const figure = myPlan?.balance.unlimited
    ? t('balance.unlimited', { ns: 'plans' })
    : myPlan && !isPending
      ? t('balance.credits', { ns: 'plans', count: myPlan.balance.credits })
      : '—'

  const arrived = useRef(false)
  useEffect(() => {
    if (arrived.current || !openPacks || !myPlan || chargeToPlans) return
    arrived.current = true
    setPacksOpen(true)
  }, [openPacks, myPlan, chargeToPlans])

  const charge = (
    <>
      <Zap aria-hidden="true" className="size-5" />
      {t('creditsCard.charge')}
    </>
  )
  return (
    <section
      aria-labelledby={headingId}
      className={cardStyles({
        padding: 'lg',
        className: `bg-badge-accent-bg animate-step-in flex flex-col gap-5 ${className ?? ''}`,
      })}
    >
      <div className="flex items-center justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1">
          <h2 id={headingId} className={typographyStyles({ variant: 'label' })}>
            {t('creditsCard.credits')}
          </h2>
          <p className={typographyStyles({ variant: 'figure' })}>{figure}</p>
          <p className={typographyStyles({ variant: 'body', className: 'text-content-secondary' })}>
            {planLabel(myPlan?.plan)}
          </p>
        </div>
        <img
          src={creditsIllustration}
          alt=""
          aria-hidden="true"
          width={80}
          height={80}
          className="size-16 shrink-0 object-contain drop-shadow-sm sm:size-20"
        />
      </div>
      {chargeToPlans ? (
        <Link
          to="/plans"
          className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full' })}
        >
          {charge}
        </Link>
      ) : (
        <Button
          variant="cta"
          size="hero"
          className="w-full"
          aria-haspopup="dialog"
          onClick={() => setPacksOpen(true)}
        >
          {charge}
        </Button>
      )}
      <Sheet
        open={packsOpen}
        label={t('creditsCard.packs')}
        onClose={() => setPacksOpen(false)}
        footer={
          <div className="flex justify-end pt-3">
            <Button variant="ghost" onClick={() => setPacksOpen(false)}>
              {t('action.close', { ns: 'common' })}
            </Button>
          </div>
        }
      >
        <Packs />
      </Sheet>
    </section>
  )
}

/** The pack purchase, bound to billing only once the sheet is up: 설정 reads no billing until
 *  the owner asks to buy, and the billing screen shares the read it already made. */
function Packs() {
  const { t } = useTranslation(['billing', 'common'])
  const { myBilling, isError } = useMyBilling()
  if (isError)
    return (
      <Notice tone="danger" role="alert">
        {t('creditsCard.packsLoadFailed')}
      </Notice>
    )
  if (!myBilling)
    return <LoadingState variant="form" label={t('state.loading', { ns: 'common' })} />
  return (
    <CreditPurchaseSection
      hasPaymentMethod={myBilling.paymentMethod !== undefined}
      activePaid={myBilling.subscription?.status === 'active'}
      purchases={myBilling.purchases}
    />
  )
}
