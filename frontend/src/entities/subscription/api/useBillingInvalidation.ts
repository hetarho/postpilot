import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey } from '@/entities/plan/@x/subscription'
import { invalidateModelAccess } from '@/entities/model-catalog/@x/subscription'
import { myBillingQueryKey } from './useMyBilling'

/** Refreshes everything a billing change can move: the billing summary, the plan and model access. */
export function useBillingInvalidation() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  return async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
      queryClient.invalidateQueries({ queryKey: myPlanQueryKey(transport) }),
      invalidateModelAccess(queryClient, transport),
    ])
  }
}
