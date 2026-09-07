import { useQuery } from '@connectrpc/connect-query'
import { PlanService } from '@/shared/api'
import { PLAN_BALANCE_STALE_MS } from '@/shared/config'
import type { MyPlan } from '../model/types'
import { toMyPlan } from './plan-mappers'

/** The caller's own tier, limits and live usage.
 *
 *  Two surfaces read this now — the header's credit control and the account popover
 *  (QUOTA-27) — so it carries a short stale window rather than refetching on every mount:
 *  opening the popover over a header that already has the figure must not cost a second
 *  request. The window is short enough that a job which actually spent credits is reflected
 *  long before the user can look. */
export function useMyPlan(enabled = true): {
  myPlan: MyPlan | undefined
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(
    PlanService.method.getMyPlan,
    {},
    { enabled, staleTime: PLAN_BALANCE_STALE_MS },
  )
  return { myPlan: toMyPlan(data), isPending, isError }
}
