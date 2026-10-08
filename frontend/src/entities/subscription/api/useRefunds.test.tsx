import { createRouterTransport } from '@connectrpc/connect'
import { act, cleanup, renderHook, waitFor } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { BillingService } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import {
  useMyRefunds,
  useRefundReviews,
  useRequestRefund,
  useReviewRefund,
  useReconcileRefund,
} from './useRefunds'

afterEach(cleanup)

it('maps list observers, refetches and named mutation results at the same domain seam', async () => {
  const request = {
    id: 'refund',
    status: 'requested',
    payment: { orderId: 'order', chargedKrw: 9007199254740993n },
  }
  const transport = createRouterTransport(({ rpc }) => {
    rpc(BillingService.method.listMyRefunds, () => ({ refunds: [request] }))
    rpc(BillingService.method.listRefundReviews, () => ({ refunds: [request] }))
    rpc(BillingService.method.requestRefund, () => ({ refund: request }))
    rpc(BillingService.method.reviewRefund, () => ({
      refund: { ...request, status: 'processing' },
    }))
    rpc(BillingService.method.reconcileRefund, () => ({
      refund: { ...request, status: 'completed' },
    }))
  })
  const hook = renderHook(
    () => ({
      mine: useMyRefunds(),
      reviews: useRefundReviews(),
      request: useRequestRefund(),
      review: useReviewRefund(),
      reconcile: useReconcileRefund(),
    }),
    { wrapper: withProviders(transport, createTestQueryClient()) },
  )
  await waitFor(() => expect(hook.result.current.mine.refunds).toHaveLength(1))
  await waitFor(() => expect(hook.result.current.reviews.refunds).toHaveLength(1))
  for (const list of [hook.result.current.mine.data, hook.result.current.reviews.data]) {
    expect(list?.[0]).not.toHaveProperty('$typeName')
    expect(list?.[0].payment?.chargedKrw).toBe(9007199254740993n)
  }
  const refreshed = await hook.result.current.mine.refetch()
  expect(refreshed.data?.[0]).not.toHaveProperty('$typeName')
  await act(async () => {
    const requested = await hook.result.current.request.requestRefund('order', 'review')
    const reviewed = await hook.result.current.review.reviewRefund('refund', 'approve', 1n)
    const completed = await hook.result.current.reconcile.reconcileRefund('refund')
    expect(requested?.status).toBe('requested')
    expect(reviewed?.status).toBe('processing')
    expect(completed?.status).toBe('completed')
    for (const value of [requested, reviewed, completed])
      expect(value).not.toHaveProperty('$typeName')
  })
})
