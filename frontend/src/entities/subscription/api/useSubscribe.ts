import { useMutation } from '@connectrpc/connect-query'
import { planToProto, type PlanName } from '@/entities/plan/@x/subscription'
import { BillingService } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import type { BillingTerm } from '../model/types'
import { termToProto } from './billing-mappers'
import { useBillingInvalidation } from './useBillingInvalidation'

export function useSubscribe() {
  const mutation = useMutation(BillingService.method.subscribe, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    subscribe: (plan: PlanName, term: BillingTerm) =>
      mutation.mutateAsync({ plan: planToProto(plan), term: termToProto(term) }),
    errorMessage: connectFailureMessage(mutation.error),
  }
}
