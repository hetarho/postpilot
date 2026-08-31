import { useQuery } from '@connectrpc/connect-query'
import { PlanService } from '@/shared/api'
import type { MyPlan } from '../model/types'
import { toMyPlan } from './plan-mappers'

/** The caller's own tier, limits and live usage.
 *
 *  `staleTime: 0` because the numbers move with every job the account starts: this is
 *  mounted inside a panel that opens on demand, so a refetch per open is exactly the cost
 *  of showing a figure the user can trust. */
export function useMyPlan(enabled = true): {
  myPlan: MyPlan | undefined
  isPending: boolean
  isError: boolean
} {
  const { data, isPending, isError } = useQuery(
    PlanService.method.getMyPlan,
    {},
    { enabled, staleTime: 0 },
  )
  return { myPlan: toMyPlan(data), isPending, isError }
}
