import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { PLANS, isPlanName, planLabel, useSetUserPlan, type PlanAccount } from '@/entities/plan'
import { AppFailureMessage, Select } from '@/shared/ui'

/** The operator's per-account tier control.
 *
 *  The server is the authority on every refusal this can produce — demoting the last master, an
 *  unknown account — so the control does not predict them: it sends the change and renders what
 *  came back. */
export function UserPlanSelect({ account }: { account: PlanAccount }) {
  const { t } = useTranslation('plans')
  const id = useId()
  const change = useSetUserPlan()

  return (
    <div className="min-w-0">
      <Select
        id={id}
        aria-label={t('admin.planFor', { id: account.id })}
        // A tier this build cannot decode selects the placeholder rather than the first rung:
        // the rest of the ladder fails closed on an unknown value, and showing it as `free`
        // would invite an operator to "confirm" a tier they were never shown.
        value={account.plan ?? ''}
        disabled={change.isPending}
        onChange={(event) => {
          const next = event.target.value
          if (isPlanName(next) && next !== account.plan) change.setPlan(account.id, next)
        }}
      >
        {!account.plan && <option value="">{planLabel(undefined)}</option>}
        {PLANS.map((plan) => (
          <option key={plan} value={plan}>
            {planLabel(plan)}
          </option>
        ))}
      </Select>
      <p role="status" className="text-content-tertiary mt-1 text-xs empty:hidden">
        {change.isPending ? t('admin.saving') : null}
      </p>
      {change.failure && (
        <div role="alert" className="text-field-error mt-1 text-xs break-words">
          <AppFailureMessage failure={change.failure} />
        </div>
      )}
    </div>
  )
}
