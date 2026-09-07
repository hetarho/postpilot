import type { PlanName } from '@/entities/plan/@x/subscription'

export type BillingTerm = 'monthly' | 'annual'

export interface Subscription {
  plan: PlanName | undefined
  term: BillingTerm | undefined
  anchorAt: string
  termStart: string
  termEnd: string
  nextGrantAt: string
  autoRenew: boolean
  scheduledPlan: PlanName | undefined
  scheduledTerm: BillingTerm | undefined
  status: string
}

export interface PaymentMethod {
  cardLabel: string
  registeredAt: string
}

export interface BillingEvent {
  id: bigint
  kind: string
  plan: PlanName | undefined
  term: BillingTerm | undefined
  credits: number
  usdCents: number
  krwPerUsdE4: bigint
  rateDate: string
  krw: bigint
  providerPaymentKey: string
  orderId: string
  note: string
  createdAt: string
}

export interface Purchase {
  id: string
  lotId: string
  credits: number
  usdCents: number
  krw: bigint
  providerPaymentKey: string
  orderId: string
  chargedAt: string
  refundedAt: string
}

export interface Quote {
  usdCents: number
  krw: bigint
  ratePerUsdE4: bigint
  rateDate: string
}

export interface ChangeQuote extends Quote {
  appliedNow: boolean
  effectiveAt: string
}

export interface MyBilling {
  subscription: Subscription | undefined
  paymentMethod: PaymentMethod | undefined
  history: BillingEvent[]
  purchases: Purchase[]
  customerKey: string
}
