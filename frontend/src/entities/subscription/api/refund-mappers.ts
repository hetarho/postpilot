import type { ProtoBillingRefundRequest } from '@/shared/api'
import type { BillingRefundRequest } from '../model/refund'

export function toRefund(value: ProtoBillingRefundRequest): BillingRefundRequest {
  return {
    id: value.id,
    userId: value.userId,
    reason: value.reason,
    status: value.status,
    requestedAt: value.requestedAt,
    reviewedBy: value.reviewedBy,
    reviewedAt: value.reviewedAt,
    reviewedAmountKrw: value.reviewedAmountKrw,
    confirmedAmountKrw: value.confirmedAmountKrw,
    confirmedAt: value.confirmedAt,
    dispositionJson: value.dispositionJson,
    priorRefundedKrw: value.priorRefundedKrw,
    payment: value.payment
      ? {
          orderId: value.payment.orderId,
          kind: value.payment.kind,
          providerPaymentKey: value.payment.providerPaymentKey,
          chargedKrw: value.payment.chargedKrw,
          chargedAt: value.payment.chargedAt,
          coverageId: value.payment.coverageId,
          packLotId: value.payment.packLotId,
        }
      : undefined,
    evidence: value.evidence
      ? {
          paidModelJobs: value.evidence.paidModelJobs,
          creditsUsed: value.evidence.creditsUsed,
          creditsReserved: value.evidence.creditsReserved,
          serverExportsUsed: value.evidence.serverExportsUsed,
          serverExportsReserved: value.evidence.serverExportsReserved,
          fundedCreditsRemaining: value.evidence.fundedCreditsRemaining,
          fundedExportsRemaining: value.evidence.fundedExportsRemaining,
        }
      : undefined,
  }
}

export function toRefundList(response: { refunds: ProtoBillingRefundRequest[] }) {
  return response.refunds.map(toRefund)
}

export function toRefundResult(response: { refund?: ProtoBillingRefundRequest }) {
  return response.refund ? toRefund(response.refund) : undefined
}
