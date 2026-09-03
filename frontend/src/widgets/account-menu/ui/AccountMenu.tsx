import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { UserRound } from 'lucide-react'
import { useMyPlan, planLabel } from '@/entities/plan'
import { useLogout, useSession } from '@/entities/session'
import {
  AppFailureMessage,
  Badge,
  Button,
  Meter,
  Notice,
  Popover,
  Typography,
  typographyStyles,
} from '@/shared/ui'

/** The header's one session control: an avatar that opens the account material — who is signed
 *  in, the plan and its meters, the operator's admin entry, and logout. Collapsing these into a
 *  popover keeps the 320px header to three quiet controls without hiding any session fact more
 *  than one tap away.
 *
 *  Logout runs here, but where a SUCCESSFUL logout lands stays the shell's decision via
 *  `onLoggedOut` — the shell owns the session cache drop and the navigation, and awaiting the
 *  mutation first is what keeps the guard from reading a stale session. A FAILED logout leaves
 *  the cookie valid, so the popover stays open and says so where the user is already looking
 *  (design-language §4.3) instead of pretending the session ended. */
export function AccountMenu({ onLoggedOut }: { onLoggedOut: () => void }) {
  const { t } = useTranslation(['auth', 'common'])
  const { user } = useSession()
  const logout = useLogout()

  const onLogout = async () => {
    try {
      await logout.mutateAsync({})
    } catch {
      return
    }
    onLoggedOut()
  }

  return (
    <Popover
      label={t('account.label', { ns: 'auth' })}
      triggerLabel={
        // The session model has no profile picture, so the header uses one stable profile glyph
        // instead of turning an arbitrary account-id character into an avatar.
        <UserRound aria-hidden="true" className="size-5" />
      }
      triggerClassName="size-11 rounded-full px-0"
      placement="below"
    >
      {(close) => (
        <div className="grid gap-4">
          <div className="grid gap-1">
            <Typography variant="label" as="p">
              {t('account.signedInAs', { ns: 'auth' })}
            </Typography>
            <Typography variant="label" as="p" mono className="text-content-primary break-words">
              {user?.id}
            </Typography>
          </div>
          <PlanPanel close={close} />
          <Button
            variant="secondary"
            className="w-full"
            onClick={() => void onLogout()}
            pending={logout.isPending}
          >
            {t('action.logout', { ns: 'common' })}
          </Button>
          {logout.failure && (
            <Notice tone="danger" role="alert">
              <AppFailureMessage failure={logout.failure} />
              <span>{t('logout.failed', { ns: 'auth' })}</span>
            </Notice>
          )}
        </div>
      )}
    </Popover>
  )
}

/** The plan badge and the three usage meters.
 *
 *  Every number comes from GetMyPlan: the grant table and the charge formula are server-owned
 *  product rules, so a figure hardcoded here would be a second source of truth that silently
 *  goes stale. The panel mounts only while the popover is open, which is what makes
 *  "refreshed when opened" fall out of the query rather than out of a timer — and reading a
 *  balance is also what renews it, so opening this at a month boundary shows the new grant. */
function PlanPanel({ close }: { close: () => void }) {
  const { t } = useTranslation('plans')
  const { myPlan, isPending, isError } = useMyPlan()

  if (isPending)
    return (
      <Typography variant="body" className="text-content-tertiary">
        {t('balance.loading')}
      </Typography>
    )
  if (isError || !myPlan)
    return (
      <Typography variant="body" role="status" className="text-content-tertiary">
        {t('balance.loadFailed')}
      </Typography>
    )

  const { balance } = myPlan
  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-2">
        <Badge tone="accent">{planLabel(myPlan.plan)}</Badge>
        <Typography variant="meta">{t('balance.assignedByOperator')}</Typography>
      </div>
      <section className="grid gap-3">
        <Typography variant="title" as="h2">
          {t('balance.heading')}
        </Typography>
        {balance.unlimited ? (
          <Typography variant="body">{t('balance.unlimited')}</Typography>
        ) : (
          <>
            {/* The meter fills against everything the account was GRANTED, not against the
                monthly figure alone: a signup bonus makes the balance larger than one
                month's grant, and a bar that read 100/50 would be nonsense. */}
            <Meter
              label={t('balance.heading')}
              value={balance.credits}
              max={
                balance.lots.reduce((total, lot) => total + lot.granted, 0) || balance.monthlyGrant
              }
              valueText={t('balance.credits', { count: balance.credits })}
              note={balance.renewsAt ? t('balance.renews', { at: balance.renewsAt }) : undefined}
            />
            {/* The lots behind the total, in the order they will be spent. A single number
                cannot say that half the balance lapses at the month boundary and half does
                not, which is the one thing a bonus holder needs to know. */}
            <ul className="grid gap-1">
              {balance.lots.map((lot, index) => (
                <li
                  key={`${lot.kind}-${index}`}
                  className="flex items-baseline justify-between gap-2"
                >
                  <Typography variant="meta">
                    {lot.kind === 'monthly' ? t('balance.lotMonthly') : t('balance.lotBonus')}
                    {' · '}
                    {lot.expiresAt
                      ? t('balance.lotExpires', { at: lot.expiresAt })
                      : t('balance.lotNoExpiry')}
                  </Typography>
                  <Typography variant="meta">
                    {t('balance.ofGrant', { remaining: lot.remaining, granted: lot.granted })}
                  </Typography>
                </li>
              ))}
            </ul>
            <Link
              to="/plans"
              onClick={close}
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2',
              })}
            >
              {t('balance.viewPlans')}
            </Link>
          </>
        )}
      </section>
      {myPlan.plan === 'master' && (
        <Link
          to="/admin"
          onClick={close}
          className={typographyStyles({
            variant: 'label',
            className:
              'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2',
          })}
        >
          {t('admin.nav')}
        </Link>
      )}
    </div>
  )
}
