export interface RefundPayment {
  orderId: string
  kind: string
  providerPaymentKey: string
  chargedKrw: bigint
  chargedAt: string
  coverageId: string
  packLotId: string
}

export interface RefundEvidence {
  paidModelJobs: number
  creditsUsed: number
  creditsReserved: number
  serverExportsUsed: number
  serverExportsReserved: number
  fundedCreditsRemaining: number
  fundedExportsRemaining: number
}

export interface BillingRefundRequest {
  id: string
  userId: string
  reason: string
  status: string
  requestedAt: string
  reviewedBy: string
  reviewedAt: string
  reviewedAmountKrw: bigint
  confirmedAmountKrw: bigint
  confirmedAt: string
  dispositionJson: string
  priorRefundedKrw: bigint
  payment: RefundPayment | undefined
  evidence: RefundEvidence | undefined
}
