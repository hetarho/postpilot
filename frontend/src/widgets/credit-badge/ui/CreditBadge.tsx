import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useMyPlan, planLabel } from '@/entities/plan'
import { typographyStyles } from '@/shared/ui'

/** The header's credit control: what is left to spend, and the way to the ladder.
 *
 *  It exists because the balance used to sit two taps behind the profile glyph and `/plans`
 *  had no other entry at all, so nothing in the app led anyone to the tiers (QUOTA-27).
 *
 *  The shell mounts this, which means the balance is read on load rather than only when a
 *  popover opens. That read is also what renews the monthly grant (QUOTA-26) — the same
 *  idempotent per-cycle insert the server already performed on the popover's read, so this
 *  moves WHEN it happens and adds no new write path. The account popover reads the same
 *  cache entry, so opening it costs no second request.
 *
 *  The figure alone is the visible label on a phone: at 320px the header already carries the
 *  wordmark and three controls, and the tier name is the first thing that has to give way.
 *  The accessible name keeps both, so nothing is lost to a screen reader. */
export function CreditBadge() {
  const { t } = useTranslation('plans')
  const { myPlan, isPending } = useMyPlan()

  const tier = planLabel(myPlan?.plan)
  const unlimited = myPlan?.balance.unlimited ?? false
  // A dash while the read is in flight, and after a failed one: the control's job is to lead
  // to `/plans`, and the popover is where a balance failure is already reported. Holding the
  // box either way keeps the header from shifting under the thumb on arrival.
  const figure = unlimited
    ? t('balance.unlimited')
    : myPlan && !isPending
      ? t('balance.credits', { count: myPlan.balance.credits })
      : '—'

  return (
    <Link
      to="/plans"
      aria-label={
        unlimited
          ? t('badge.labelUnlimited', { tier })
          : t('badge.label', { tier, count: myPlan?.balance.credits ?? 0 })
      }
      className={typographyStyles({
        variant: 'label',
        className:
          'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center gap-1.5 px-2',
      })}
    >
      <span aria-hidden="true" className="hidden sm:inline">
        {tier}
      </span>
      <span aria-hidden="true">{figure}</span>
    </Link>
  )
}
