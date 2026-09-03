// Proto ↔ domain for the plan ladder. The Plan enum crosses here and nowhere else.
import {
  type GetMyPlanResponse,
  ProtoPlan,
  type ProtoPlanUser,
  type ProtoCreditBalance,
} from '@/shared/api'
import type {
  CreditBalance,
  CreditLot,
  MyPlan,
  PlanAccount,
  PlanName,
  PlanOffer,
} from '../model/types'

const PLAN_TO_PROTO: Record<PlanName, ProtoPlan> = {
  free: ProtoPlan.FREE,
  basic: ProtoPlan.BASIC,
  pro: ProtoPlan.PRO,
  max: ProtoPlan.MAX,
  master: ProtoPlan.MASTER,
}

export function planToProto(plan: PlanName): ProtoPlan {
  return PLAN_TO_PROTO[plan]
}

/** Undefined for UNSPECIFIED and for a tier this build does not know — the caller then has
 *  no authority to show, which is the safe reading of a value it cannot interpret. */
export function planFromProto(plan: ProtoPlan): PlanName | undefined {
  for (const [name, wire] of Object.entries(PLAN_TO_PROTO) as [PlanName, ProtoPlan][]) {
    if (wire === plan) return name
  }
  return undefined
}

/** An unknown lot kind is read as a bonus: the distinction is a label, and a grant that
 *  cannot be named is still a grant the account holds. */
function toLot(kind: string): CreditLot['kind'] {
  return kind === 'monthly' ? 'monthly' : 'bonus'
}

function toBalance(balance: ProtoCreditBalance | undefined): CreditBalance {
  return {
    credits: balance?.credits ?? 0,
    unlimited: balance?.unlimited ?? false,
    lots: (balance?.lots ?? []).map((lot) => ({
      kind: toLot(lot.kind),
      granted: lot.granted,
      remaining: lot.remaining,
      expiresAt: lot.expiresAt,
    })),
    renewsAt: balance?.renewsAt ?? '',
    monthlyGrant: balance?.monthlyGrant ?? 0,
  }
}

function toOffer(offer: {
  plan: ProtoPlan
  monthlyCredits: number
  priceUsdCents: number
}): PlanOffer {
  return {
    plan: planFromProto(offer.plan),
    monthlyCredits: offer.monthlyCredits,
    priceUsdCents: offer.priceUsdCents,
  }
}

export function toMyPlan(response: GetMyPlanResponse | undefined): MyPlan | undefined {
  if (!response) return undefined
  return {
    plan: planFromProto(response.plan),
    balance: toBalance(response.balance),
    // A tier this build cannot name is dropped rather than rendered as unknown: an offer
    // nobody can identify is not something to put a price next to.
    offers: (response.offers ?? []).map(toOffer).filter((offer) => offer.plan !== undefined),
  }
}

export function toPlanAccount(user: ProtoPlanUser): PlanAccount {
  return { id: user.id, plan: planFromProto(user.plan), createdAt: user.createdAt }
}
