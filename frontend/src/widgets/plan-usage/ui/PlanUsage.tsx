import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { Gauge } from 'lucide-react'
import { useMyPlan, planLabel, type PlanName } from '@/entities/plan'
import { formatMicroUsd } from '@/shared/lib'
import { Badge, Meter, Popover } from '@/shared/ui'

/** The account area's plan badge and the three usage meters behind it.
 *
 *  Every number comes from GetMyPlan: the limits are a server-owned product rule, so a figure
 *  hardcoded here would be a second source of truth that silently goes stale after a tier change.
 *  The panel's contents mount only while it is open, which is what makes "refreshed when opened"
 *  fall out of the query rather than out of a timer. */
export function PlanUsage({ plan }: { plan: PlanName | undefined }) {
  const { t } = useTranslation('plans')
  return (
    <Popover
      label={t('badge.label', { plan: planLabel(plan) })}
      triggerLabel={
        <>
          <Gauge aria-hidden="true" className="size-7 sm:hidden" />
          <Badge tone="accent" className="hidden sm:inline-flex">
            {planLabel(plan)}
          </Badge>
        </>
      }
      triggerClassName="px-3 sm:px-2"
      placement="below"
    >
      {() => <PlanUsagePanel />}
    </Popover>
  )
}

function PlanUsagePanel() {
  const { t } = useTranslation('plans')
  const { myPlan, isPending, isError } = useMyPlan()

  if (isPending) return <p className="text-content-tertiary text-sm">{t('usage.loading')}</p>
  if (isError || !myPlan)
    return (
      <p role="status" className="text-content-tertiary text-sm">
        {t('usage.loadFailed')}
      </p>
    )

  const { limits, usage } = myPlan
  return (
    <div className="grid gap-4">
      <div className="flex items-center justify-between gap-2">
        <Badge tone="accent">{planLabel(myPlan.plan)}</Badge>
        <span className="text-content-tertiary text-xs">{t('usage.assignedByOperator')}</span>
      </div>
      <section className="grid gap-3">
        <h2 className="text-content-secondary text-sm font-medium">{t('usage.heading')}</h2>
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
        <Link to="/admin" className="text-link-fg hover:text-link-fg-hover inline-flex text-sm">
          {t('admin.nav')}
        </Link>
      )}
    </div>
  )
}
