import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { BillingService } from '@/shared/api'

export function useMyRefunds() {
  const query = useQuery(BillingService.method.listMyRefunds, {})
  return { ...query, refunds: query.data?.refunds ?? [] }
}

export function useRefundReviews() {
  const query = useQuery(BillingService.method.listRefundReviews, {})
  return { ...query, refunds: query.data?.refunds ?? [] }
}

function useRefundInvalidation() {
  const transport = useTransport()
  const client = useQueryClient()
  return async () => {
    await Promise.all([
      client.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: BillingService.method.listMyRefunds,
          input: {},
          transport,
          cardinality: 'finite',
        }),
      }),
      client.invalidateQueries({
        queryKey: createConnectQueryKey({
          schema: BillingService.method.listRefundReviews,
          input: {},
          transport,
          cardinality: 'finite',
        }),
      }),
    ])
  }
}

export function useRequestRefund() {
  const mutation = useMutation(BillingService.method.requestRefund, {
    onSuccess: useRefundInvalidation(),
  })
  return {
    ...mutation,
    requestRefund: (orderId: string, reason: string) => mutation.mutateAsync({ orderId, reason }),
  }
}

export function useReviewRefund() {
  const mutation = useMutation(BillingService.method.reviewRefund, {
    onSuccess: useRefundInvalidation(),
  })
  return {
    ...mutation,
    reviewRefund: (requestId: string, outcome: string, reviewedAmountKrw: bigint) =>
      mutation.mutateAsync({ requestId, outcome, reviewedAmountKrw }),
  }
}

export function useReconcileRefund() {
  const mutation = useMutation(BillingService.method.reconcileRefund, {
    onSuccess: useRefundInvalidation(),
  })
  return {
    ...mutation,
    reconcileRefund: (requestId: string) => mutation.mutateAsync({ requestId }),
  }
}
