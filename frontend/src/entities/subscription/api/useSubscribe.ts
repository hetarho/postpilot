import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey, planToProto, type PlanName } from '@/entities/plan/@x/subscription'
import { appFailureFromConnect, BillingService } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { BillingTerm } from '../model/types'
import { termToProto } from './billing-mappers'
import { myBillingQueryKey } from './useMyBilling'

export function useSubscribe() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const mutation = useMutation(BillingService.method.subscribe, {
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
        queryClient.invalidateQueries({ queryKey: myPlanQueryKey(transport) }),
      ])
    },
  })
  return {
    ...mutation,
    subscribe: (plan: PlanName, term: BillingTerm) =>
      mutation.mutateAsync({ plan: planToProto(plan), term: termToProto(term) }),
    errorMessage: mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : '',
  }
}
