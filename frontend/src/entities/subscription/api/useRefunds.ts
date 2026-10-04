import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, BillingService } from '@/shared/api'

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

/** The operator's decision on one request. The lists refresh when it settles, not only when it
 *  succeeds: an approval whose cancel the provider refused has still decided the request, and
 *  the row must show it failed beside the refusal (REFUND_FAILED). */
export function useReviewRefund() {
  const mutation = useMutation(BillingService.method.reviewRefund, {
    onSettled: useRefundInvalidation(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    reviewRefund: (requestId: string, outcome: string, reviewedAmountKrw: bigint) =>
      mutation.mutateAsync({ requestId, outcome, reviewedAmountKrw }),
  }
}

/** The operator's re-check of a processing request, refreshed on settle for the same reason. */
export function useReconcileRefund() {
  const mutation = useMutation(BillingService.method.reconcileRefund, {
    onSettled: useRefundInvalidation(),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    reconcileRefund: (requestId: string) => mutation.mutateAsync({ requestId }),
  }
}
