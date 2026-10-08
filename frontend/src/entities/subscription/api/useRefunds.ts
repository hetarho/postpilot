import {
  createConnectQueryKey,
  useMutation,
  useQuery,
  useTransport,
} from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, BillingService } from '@/shared/api'
import { toRefundList, toRefundResult } from './refund-mappers'

export function useMyRefunds() {
  const query = useQuery(BillingService.method.listMyRefunds, {}, { select: toRefundList })
  return { ...query, refunds: query.data ?? [] }
}

export function useRefundReviews() {
  const query = useQuery(BillingService.method.listRefundReviews, {}, { select: toRefundList })
  return { ...query, refunds: query.data ?? [] }
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
    isPending: mutation.isPending,
    isError: mutation.isError,
    isSuccess: mutation.isSuccess,
    reset: mutation.reset,
    requestRefund: async (orderId: string, reason: string) =>
      toRefundResult(await mutation.mutateAsync({ orderId, reason })),
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
    isPending: mutation.isPending,
    isError: mutation.isError,
    isSuccess: mutation.isSuccess,
    reset: mutation.reset,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    reviewRefund: async (requestId: string, outcome: string, reviewedAmountKrw: bigint) =>
      toRefundResult(await mutation.mutateAsync({ requestId, outcome, reviewedAmountKrw })),
  }
}

/** The operator's re-check of a processing request, refreshed on settle for the same reason. */
export function useReconcileRefund() {
  const mutation = useMutation(BillingService.method.reconcileRefund, {
    onSettled: useRefundInvalidation(),
  })
  return {
    isPending: mutation.isPending,
    isError: mutation.isError,
    isSuccess: mutation.isSuccess,
    reset: mutation.reset,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
    reconcileRefund: async (requestId: string) =>
      toRefundResult(await mutation.mutateAsync({ requestId })),
  }
}
