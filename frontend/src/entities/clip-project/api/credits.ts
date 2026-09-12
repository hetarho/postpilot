import type { ProtoClipAccounting, ProtoClipQuote } from '@/shared/api'
import type { ClipAccounting, ClipQuote } from '../model/types'

const STATUSES = [
  'not_reserved',
  'reserved',
  'settling',
  'settled',
  'exempt',
  'unavailable',
] as const
export function toClipAccounting(value: ProtoClipAccounting): ClipAccounting {
  const amount = (n: number | undefined) =>
    n !== undefined && Number.isSafeInteger(n) && n >= 0 ? n : undefined
  const status = STATUSES.includes(value.status as ClipAccounting['status'])
    ? (value.status as ClipAccounting['status'])
    : 'unavailable'
  const finalChargeCredits = amount(value.finalChargeCredits),
    refundCredits = amount(value.refundCredits)
  return {
    nominalReservedCredits: amount(value.nominalReservedCredits),
    confirmedChargeCredits: amount(value.confirmedChargeCredits),
    cancellationFeeCredits: amount(value.cancellationFeeCredits),
    shadowConfirmedChargeCredits: amount(value.shadowConfirmedChargeCredits),
    shadowCancellationFeeCredits: amount(value.shadowCancellationFeeCredits),
    settlementReason: ['succeeded', 'failed', 'cancelled'].includes(value.settlementReason)
      ? (value.settlementReason as ClipAccounting['settlementReason'])
      : undefined,
    cancellationPolicyVersion: amount(value.cancellationPolicyVersion),
    jobId: value.jobId,
    status,
    approvedMaxCredits: amount(value.approvedMaxCredits),
    reservedCredits: amount(value.reservedCredits),
    finalChargeCredits,
    refundCredits,
    shadowChargeCredits: amount(value.shadowChargeCredits),
    settled:
      value.settled &&
      status !== 'unavailable' &&
      finalChargeCredits !== undefined &&
      refundCredits !== undefined,
  }
}
export function toClipQuote(value: ProtoClipQuote, binding: string): ClipQuote {
  if (
    !value.quoteId ||
    !Number.isSafeInteger(value.maxCredits) ||
    value.maxCredits < 0 ||
    !Number.isFinite(Date.parse(value.expiresAt))
  )
    throw new Error('Invalid clip quote')
  const policy = value.cancellationPolicy
  if (
    policy &&
    (policy.version !== 1 ||
      policy.unusedReservationNumerator !== 1 ||
      policy.unusedReservationDenominator !== 2 ||
      policy.rounding !== 'ceil')
  )
    throw new Error('Unsupported clip cancellation policy')
  return {
    ...(policy
      ? {
          cancellationPolicy: {
            version: policy.version,
            numerator: policy.unusedReservationNumerator,
            denominator: policy.unusedReservationDenominator,
            rounding: 'ceil' as const,
          },
        }
      : {}),
    quoteId: value.quoteId,
    maxCredits: value.maxCredits,
    expiresAt: value.expiresAt,
    binding,
  }
}
