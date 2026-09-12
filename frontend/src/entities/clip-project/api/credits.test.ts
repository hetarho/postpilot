import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { ClipAccountingSchema, QuoteClipGenerationResponseSchema } from '@/shared/api'
import { toClipAccounting, toClipQuote } from './credits'

it('distinguishes absent amounts from explicit zero and requires complete settlement', () => {
  const missing = toClipAccounting(
    create(ClipAccountingSchema, { jobId: 'job', status: 'settling' }),
  )
  expect(missing.finalChargeCredits).toBeUndefined()
  expect(missing.refundCredits).toBeUndefined()
  expect(missing.settled).toBe(false)
  const zero = toClipAccounting(
    create(ClipAccountingSchema, {
      jobId: 'job',
      status: 'not_reserved',
      finalChargeCredits: 0,
      refundCredits: 0,
      settled: true,
    }),
  )
  expect(zero.finalChargeCredits).toBe(0)
  expect(zero.refundCredits).toBe(0)
  expect(zero.settled).toBe(true)
  for (const raw of [
    { status: 'future', finalChargeCredits: 0, refundCredits: 0 },
    { status: 'settled', finalChargeCredits: -1, refundCredits: 5 },
    { status: 'settled', finalChargeCredits: 0 },
  ]) {
    expect(toClipAccounting(create(ClipAccountingSchema, { ...raw, settled: true })).settled).toBe(
      false,
    )
  }
})
it('treats the server ceiling as opaque, including zero, and rejects malformed quotes', () => {
  const raw = { quoteId: 'quote', maxCredits: 0, expiresAt: '2099-01-01T00:00:00Z' }
  expect(toClipQuote(create(QuoteClipGenerationResponseSchema, raw), 'bound')).toEqual({
    ...raw,
    binding: 'bound',
  })
  for (const change of [
    { quoteId: '' },
    { maxCredits: -1 },
    { maxCredits: NaN },
    { expiresAt: 'invalid' },
  ]) {
    expect(() =>
      toClipQuote(create(QuoteClipGenerationResponseSchema, { ...raw, ...change }), 'bound'),
    ).toThrow('Invalid clip quote')
  }
})

it('preserves cancellation breakdowns and rejects an undisclosed policy version', () => {
  const old = toClipAccounting(
    create(ClipAccountingSchema, {
      jobId: 'old',
      status: 'settled',
      finalChargeCredits: 2,
      refundCredits: 3,
      settled: true,
    }),
  )
  expect(old.confirmedChargeCredits).toBeUndefined()
  expect(old.cancellationFeeCredits).toBeUndefined()
  const cancelled = toClipAccounting(
    create(ClipAccountingSchema, {
      jobId: 'job',
      status: 'settled',
      nominalReservedCredits: 100,
      confirmedChargeCredits: 20,
      cancellationFeeCredits: 40,
      finalChargeCredits: 60,
      refundCredits: 40,
      settlementReason: 'cancelled',
      cancellationPolicyVersion: 1,
      settled: true,
    }),
  )
  expect(cancelled).toMatchObject({
    nominalReservedCredits: 100,
    confirmedChargeCredits: 20,
    cancellationFeeCredits: 40,
    finalChargeCredits: 60,
    refundCredits: 40,
    settlementReason: 'cancelled',
  })
  const quote = create(QuoteClipGenerationResponseSchema, {
    quoteId: 'q',
    maxCredits: 5,
    expiresAt: '2099-01-01T00:00:00Z',
    cancellationPolicy: {
      version: 1,
      unusedReservationNumerator: 1,
      unusedReservationDenominator: 2,
      rounding: 'ceil',
    },
  })
  expect(toClipQuote(quote, 'binding').cancellationPolicy).toEqual({
    version: 1,
    numerator: 1,
    denominator: 2,
    rounding: 'ceil',
  })
  quote.cancellationPolicy!.version = 2
  expect(() => toClipQuote(quote, 'binding')).toThrow('Unsupported clip cancellation policy')
})
