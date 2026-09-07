import { useMutation, useQuery, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { myPlanQueryKey, planToProto, type PlanName } from '@/entities/plan/@x/subscription'
import { appFailureFromConnect, BillingService, ProtoPlan, ProtoTerm } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import type { BillingTerm } from '../model/types'
import { billablePlan, termToProto, toChangeQuote } from './billing-mappers'
import { myBillingQueryKey } from './useMyBilling'

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

function useBillingInvalidation() {
  const transport = useTransport()
  const queryClient = useQueryClient()
  return async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: myBillingQueryKey(transport) }),
      queryClient.invalidateQueries({ queryKey: myPlanQueryKey(transport) }),
    ])
  }
}

function exposeError(mutation: { error: unknown }) {
  return mutation.error ? formatAppFailure(appFailureFromConnect(mutation.error)) : ''
}

export function useChangeSubscription() {
  const invalidate = useBillingInvalidation()
  const mutation = useMutation(BillingService.method.changeSubscription, {
    onSuccess: invalidate,
  })
  return {
    ...mutation,
    changeSubscription: (plan: PlanName, term: BillingTerm) =>
      mutation.mutateAsync({ plan: planToProto(plan), term: termToProto(term) }),
    errorMessage: exposeError(mutation),
  }
}

export function useCancelScheduledChange() {
  const mutation = useMutation(BillingService.method.cancelScheduledChange, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    cancelScheduledChange: () => mutation.mutateAsync({}),
    errorMessage: exposeError(mutation),
  }
}

export function useCancelSubscription() {
  const mutation = useMutation(BillingService.method.cancelSubscription, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    cancelSubscription: () => mutation.mutateAsync({}),
    errorMessage: exposeError(mutation),
  }
}

export function useResumeSubscription() {
  const mutation = useMutation(BillingService.method.resumeSubscription, {
    onSuccess: useBillingInvalidation(),
  })
  return {
    ...mutation,
    resumeSubscription: () => mutation.mutateAsync({}),
    errorMessage: exposeError(mutation),
  }
}
