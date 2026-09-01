import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useMyPlan, planLabel } from '@/entities/plan'
import { useLogout, useSession } from '@/entities/session'
import { formatMicroUsd } from '@/shared/lib'
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
        // The session model has no display name or picture, so the avatar is the id's first
        // character; the accessible name stays the translated label on the trigger itself.
        <span aria-hidden="true">{(user?.id ?? '?').charAt(0).toUpperCase()}</span>
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
 *  Every number comes from GetMyPlan: the limits are a server-owned product rule, so a figure
 *  hardcoded here would be a second source of truth that silently goes stale after a tier change.
 *  The panel mounts only while the popover is open, which is what makes "refreshed when opened"
 *  fall out of the query rather than out of a timer. */
function PlanPanel({ close }: { close: () => void }) {
  const { t } = useTranslation('plans')
  const { myPlan, isPending, isError } = useMyPlan()

  if (isPending)
    return (
      <Typography variant="body" className="text-content-tertiary">
        {t('usage.loading')}
      </Typography>
    )
  if (isError || !myPlan)
    return (
      <Typography variant="body" role="status" className="text-content-tertiary">
        {t('usage.loadFailed')}
      </Typography>
    )

  const { limits, usage } = myPlan
  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-2">
        <Badge tone="accent">{planLabel(myPlan.plan)}</Badge>
        <Typography variant="meta">{t('usage.assignedByOperator')}</Typography>
      </div>
      <section className="grid gap-3">
        <Typography variant="title" as="h2">
          {t('usage.heading')}
        </Typography>
        <Meter
          label={t('usage.dailyStarts')}
          value={usage.jobsStartedToday}
          max={limits.dailyJobStarts}
          valueText={
            limits.dailyJobStarts > 0
              ? t('usage.ofLimit', { used: usage.jobsStartedToday, limit: limits.dailyJobStarts })
              : t('usage.unlimited', { used: usage.jobsStartedToday })
          }
          note={usage.dayResetsAt ? t('usage.dayResets', { at: usage.dayResetsAt }) : undefined}
        />
        <Meter
          label={t('usage.dailyBudget')}
          value={usage.costTodayMicrousd}
          max={limits.dailyBudgetMicrousd}
          valueText={
            limits.dailyBudgetMicrousd > 0
              ? t('usage.ofLimit', {
                  used: formatMicroUsd(usage.costTodayMicrousd),
                  limit: formatMicroUsd(limits.dailyBudgetMicrousd),
                })
              : t('usage.unlimited', { used: formatMicroUsd(usage.costTodayMicrousd) })
          }
        />
        <Meter
          label={t('usage.monthlyBudget')}
          value={usage.costMonthMicrousd}
          max={limits.monthlyBudgetMicrousd}
          valueText={
            limits.monthlyBudgetMicrousd > 0
              ? t('usage.ofLimit', {
                  used: formatMicroUsd(usage.costMonthMicrousd),
                  limit: formatMicroUsd(limits.monthlyBudgetMicrousd),
                })
              : t('usage.unlimited', { used: formatMicroUsd(usage.costMonthMicrousd) })
          }
          note={
            usage.monthResetsAt ? t('usage.monthResets', { at: usage.monthResetsAt }) : undefined
          }
        />
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
