import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BillingService } from '@/shared/api'
import { toRefund, toRefundList, toRefundResult } from './refund-mappers'

it('keeps nested absence and future status values without inventing a decision', () => {
  const response = create(BillingService.method.requestRefund.output, {
    refund: { id: 'refund', status: 'future-provider-state' },
  })
  const mapped = toRefund(response.refund!)
  expect(mapped).toMatchObject({
    id: 'refund',
    status: 'future-provider-state',
    payment: undefined,
    evidence: undefined,
    confirmedAmountKrw: 0n,
  })
  expect(toRefundResult({})).toBeUndefined()
  expect(toRefundList({ refunds: [] })).toEqual([])
})

it('preserves exact money and independent evidence without carrying wire metadata', () => {
  const amount = 9007199254740993n
  const response = create(BillingService.method.requestRefund.output, {
    refund: {
      id: 'refund',
      userId: 'owner',
      reason: 'review',
      status: 'completed',
      requestedAt: 'requested',
      reviewedBy: 'operator',
      reviewedAt: 'reviewed',
      reviewedAmountKrw: amount,
      confirmedAmountKrw: amount,
      confirmedAt: 'confirmed',
      priorRefundedKrw: 3n,
      dispositionJson: '{"effect":"void"}',
      payment: {
        orderId: 'order',
        kind: 'pack',
        providerPaymentKey: 'payment',
        chargedKrw: amount + 3n,
        chargedAt: 'charged',
        coverageId: 'coverage',
        packLotId: 'lot',
      },
      evidence: {
        paidModelJobs: 2,
        creditsUsed: 3,
        creditsReserved: 4,
        serverExportsUsed: 5,
        serverExportsReserved: 6,
        fundedCreditsRemaining: 7,
        fundedExportsRemaining: 8,
      },
    },
  })
  const original = response.refund!
  const mapped = toRefundResult(response)!
  expect(mapped).toMatchObject({
    id: 'refund',
    userId: 'owner',
    status: 'completed',
    reviewedAmountKrw: amount,
    confirmedAmountKrw: amount,
    priorRefundedKrw: 3n,
    payment: { chargedKrw: amount + 3n, packLotId: 'lot' },
    evidence: { paidModelJobs: 2, creditsReserved: 4, fundedExportsRemaining: 8 },
  })
  for (const value of [mapped, mapped.payment, mapped.evidence]) {
    expect(value).not.toHaveProperty('$typeName')
    expect(value).not.toHaveProperty('$unknown')
  }
  mapped.payment!.chargedKrw = 1n
  mapped.evidence!.creditsReserved = 0
  expect(original.payment?.chargedKrw).toBe(amount + 3n)
  expect(original.evidence?.creditsReserved).toBe(4)
})
