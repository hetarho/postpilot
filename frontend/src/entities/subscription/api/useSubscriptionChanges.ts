import { useMutation, useQuery } from '@connectrpc/connect-query'
import { planToProto, type PlanName } from '@/entities/plan/@x/subscription'
import { BillingService, ProtoPlan, ProtoTerm } from '@/shared/api'
import { connectFailureMessage } from '@/shared/lib'
import type { BillingTerm } from '../model/types'
import { billablePlan, termToProto, toChangeQuote } from './billing-mappers'
import { useBillingInvalidation } from './useBillingInvalidation'

export function useQuoteChange(plan: PlanName | undefined, term: BillingTerm | undefined) {
  const enabled = billablePlan(plan) && term !== undefined
  const query = useQuery(
    BillingService.method.quoteChange,
    {
      plan: enabled ? planToProto(plan) : ProtoPlan.UNSPECIFIED,
      term: enabled ? termToProto(term) : ProtoTerm.UNSPECIFIED,
    },
    { enabled },
  )
  return { ...query, quote: toChangeQuote(query.data) }
}

export function useChangeSubscription() {
  const invalidate = useBillingInvalidation()
  const mutation = useMutation(BillingService.method.changeSubscription, {
    onSuccess: invalidate,
  })
  return {
    ...mutation,
    changeSubscription: (plan: PlanName, term: BillingTerm, quoteId = '') =>
      mutation.mutateAsync({ plan: planToProto(plan), term: termToProto(term), quoteId }),
    errorMessage: connectFailureMessage(mutation.error),
  }
}

export function useCancelScheduledChange() {
  const mutation = useMutation(BillingService.method.cancelScheduledChange, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    cancelScheduledChange: () => mutation.mutateAsync({}),
    errorMessage: connectFailureMessage(mutation.error),
  }
}

export function useCancelSubscription() {
  const mutation = useMutation(BillingService.method.cancelSubscription, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    cancelSubscription: () => mutation.mutateAsync({}),
    errorMessage: connectFailureMessage(mutation.error),
  }
}

export function useResumeSubscription() {
  const mutation = useMutation(BillingService.method.resumeSubscription, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    resumeSubscription: () => mutation.mutateAsync({}),
    errorMessage: connectFailureMessage(mutation.error),
  }
}
