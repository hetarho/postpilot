import type { ProtoClipAccounting, ProtoClipQuote, ProtoClipRevisionQuote } from '@/shared/api'
import type { ClipAccounting, ClipPricedCall, ClipQuote } from '../model/types'

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
    !Number.isInteger(value.reusedChunks) ||
    value.reusedChunks < 0 ||
    value.reusedChunks > 49 ||
    !Number.isInteger(value.remainingChunks) ||
    value.remainingChunks < 0 ||
    value.remainingChunks > 49 ||
    ![0, 3].includes(value.responseRetries) ||
    (value.renderOnly && (value.remainingChunks !== 0 || value.maxCredits !== 0)) ||
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
    ...(value.reusedChunks || value.remainingChunks || value.renderOnly || value.responseRetries
      ? {
          recovery: {
            reusedChunks: value.reusedChunks,
            remainingChunks: value.remainingChunks,
            renderOnly: value.renderOnly,
            responseRetries: value.responseRetries,
          },
        }
      : {}),
    calls: value.pricedCalls
      .filter((call): call is typeof call & { label: ClipPricedCall['label'] } =>
        ['observe', 'flow', 'narration'].includes(call.label),
      )
      .map((call) => ({ label: call.label, calls: call.calls })),
    quoteId: value.quoteId,
    maxCredits: value.maxCredits,
    expiresAt: value.expiresAt,
    binding,
  }
}

/** A revision is quoted on its own message: no chunk recovery to report, because
 *  it does no media work at all, and a plan revision it is bound to (CLIP-131).
 *  Everything else — the ceiling, the writing calls, the cancellation rule — is
 *  the generation's contract, so it is read the same way. */
export function toClipRevisionQuote(value: ProtoClipRevisionQuote, binding: string): ClipQuote {
  if (
    !value.quoteId ||
    !Number.isSafeInteger(value.maxCredits) ||
    value.maxCredits < 0 ||
    ![0, 3].includes(value.responseRetries) ||
    !Number.isInteger(value.planRevision) ||
    value.planRevision <= 0 ||
    !Number.isFinite(Date.parse(value.expiresAt))
  )
    throw new Error('Invalid clip revision quote')
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
    calls: value.pricedCalls
      .filter((call): call is typeof call & { label: ClipPricedCall['label'] } =>
        ['observe', 'flow', 'narration'].includes(call.label),
      )
      .map((call) => ({ label: call.label, calls: call.calls })),
    quoteId: value.quoteId,
    maxCredits: value.maxCredits,
    expiresAt: value.expiresAt,
    binding,
  }
}
