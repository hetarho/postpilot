import { useTranslation } from 'react-i18next'
import {
  PLANS,
  isPlanName,
  planLabel,
  useSetUserPlan,
  type PlanAccount,
  type PlanName,
} from '@/entities/plan'
import { AppFailureMessage, SegmentedControl, Typography } from '@/shared/ui'

/** The operator's per-account tier control.
 *
 *  A bounded switch of three tiers, so every rung is on screen at once (design-language §7): an
 *  operator scanning a list of accounts is comparing tiers, and a closed dropdown hides the two
 *  they are comparing against.
 *
 *  The server is the authority on every refusal this can produce — demoting the last master, an
 *  unknown account — so the control does not predict them: it sends the change and renders what
 *  came back. */
export function UserPlanSelect({ account }: { account: PlanAccount }) {
  const { t } = useTranslation('plans')
  const change = useSetUserPlan()

  return (
    <div className="min-w-0">
      <SegmentedControl<PlanName | ''>
        ariaLabel={t('admin.planFor', { id: account.id })}
        // A tier this build cannot decode selects a placeholder rung rather than the first real
        // one: the rest of the ladder fails closed on an unknown value, and showing it as `free`
        // would invite an operator to "confirm" a tier they were never shown. Picking it back is
        // refused below, since '' is not a plan name.
        value={account.plan ?? ''}
        options={[
          ...(account.plan ? [] : [{ value: '' as const, label: planLabel(undefined) }]),
          ...PLANS.map((plan) => ({ value: plan, label: planLabel(plan) })),
        ]}
        disabled={change.isPending}
        onChange={(next) => {
          if (isPlanName(next) && next !== account.plan) change.setPlan(account.id, next)
        }}
        className="mt-3"
      />
      <Typography
        variant="body"
        as="p"
        role="status"
        className="text-content-tertiary mt-1 empty:hidden"
      >
        {change.isPending ? t('admin.saving') : null}
      </Typography>
      {change.failure && (
        <Typography
          variant="body"
          as="div"
          role="alert"
          className="text-field-error mt-1 break-words"
        >
          <AppFailureMessage failure={change.failure} />
        </Typography>
      )}
    </div>
  )
}
