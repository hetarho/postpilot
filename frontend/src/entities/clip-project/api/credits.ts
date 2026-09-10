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
  return {
    quoteId: value.quoteId,
    maxCredits: value.maxCredits,
    expiresAt: value.expiresAt,
    binding,
  }
}
