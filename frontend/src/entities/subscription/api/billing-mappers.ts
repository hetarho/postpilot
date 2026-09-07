import { planFromProto, type PlanName } from '@/entities/plan/@x/subscription'
import {
  ProtoTerm,
  type GetMyBillingResponse,
  type ProtoBillingEvent,
  type ProtoBillingPurchase,
  type ProtoBillingSubscription,
  type QuoteChangeResponse,
  type QuotePriceResponse,
} from '@/shared/api'
import type {
  BillingEvent,
  BillingTerm,
  ChangeQuote,
  MyBilling,
  Purchase,
  Quote,
  Subscription,
} from '../model/types'

export function termFromProto(term: ProtoTerm): BillingTerm | undefined {
  if (term === ProtoTerm.MONTHLY) return 'monthly'
  if (term === ProtoTerm.ANNUAL) return 'annual'
  return undefined
}

export function termToProto(term: BillingTerm): ProtoTerm {
  return term === 'annual' ? ProtoTerm.ANNUAL : ProtoTerm.MONTHLY
}

export function toSubscription(value: ProtoBillingSubscription): Subscription {
  return {
    plan: planFromProto(value.plan),
    term: termFromProto(value.term),
    anchorAt: value.anchorAt,
    termStart: value.termStart,
    termEnd: value.termEnd,
    nextGrantAt: value.nextGrantAt,
    autoRenew: value.autoRenew,
    scheduledPlan: planFromProto(value.scheduledPlan),
    scheduledTerm: termFromProto(value.scheduledTerm),
    status: value.status,
  }
}

export function toChangeQuote(response: QuoteChangeResponse | undefined): ChangeQuote | undefined {
  if (!response) return undefined
  return {
    usdCents: response.usdCents,
    krw: response.krw,
    ratePerUsdE4: response.krwPerUsdE4,
    rateDate: response.rateDate,
    appliedNow: response.appliedNow,
    effectiveAt: response.effectiveAt,
  }
}

function toEvent(value: ProtoBillingEvent): BillingEvent {
  return {
    id: value.id,
    kind: value.kind,
    plan: planFromProto(value.plan),
    term: termFromProto(value.term),
    credits: value.credits,
    usdCents: value.usdCents,
    krwPerUsdE4: value.krwPerUsdE4,
    rateDate: value.rateDate,
    krw: value.krw,
    providerPaymentKey: value.providerPaymentKey,
    orderId: value.orderId,
    note: value.note,
    createdAt: value.createdAt,
  }
}

function toPurchase(value: ProtoBillingPurchase): Purchase {
  return {
    id: value.id,
    lotId: value.lotId,
    credits: value.credits,
    usdCents: value.usdCents,
    krw: value.krw,
    providerPaymentKey: value.providerPaymentKey,
    orderId: value.orderId,
    chargedAt: value.chargedAt,
    refundedAt: value.refundedAt,
  }
}

export function toMyBilling(response: GetMyBillingResponse | undefined): MyBilling | undefined {
  if (!response) return undefined
  return {
    subscription: response.subscription ? toSubscription(response.subscription) : undefined,
    paymentMethod: response.paymentMethod
      ? {
          cardLabel: response.paymentMethod.cardLabel,
          registeredAt: response.paymentMethod.registeredAt,
        }
      : undefined,
    history: response.history.map(toEvent),
    purchases: response.purchases.map(toPurchase),
    customerKey: response.customerKey,
  }
}

export function toQuote(response: QuotePriceResponse | undefined): Quote | undefined {
  if (!response) return undefined
  return {
    usdCents: response.usdCents,
    krw: response.krw,
    ratePerUsdE4: response.krwPerUsdE4,
    rateDate: response.rateDate,
  }
}

export function billablePlan(
  plan: PlanName | undefined,
): plan is Exclude<PlanName, 'free' | 'master'> {
  return plan === 'basic' || plan === 'pro' || plan === 'max'
}
