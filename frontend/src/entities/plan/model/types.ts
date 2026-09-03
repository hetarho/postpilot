import i18next from 'i18next'

/** The ladder, in order. A tier decides two things and no more: how many credits it is
 *  granted each month, and — for `master` alone — access to the operator-only surfaces.
 *  Which models an account may run is not one of them; that is decided by what it can
 *  afford. */
export const PLANS = ['free', 'basic', 'pro', 'max', 'master'] as const

export type PlanName = (typeof PLANS)[number]

export function isPlanName(value: unknown): value is PlanName {
  return typeof value === 'string' && (PLANS as readonly string[]).includes(value)
}

/** The tier's own name for a badge or a picker. An unknown tier is named as unknown rather
 *  than as `free`: the client must never invent an authority it was not told about. */
export function planLabel(plan: PlanName | undefined): string {
  return i18next.t(`tier.${plan ?? 'unknown'}`, { ns: 'plans' })
}

/** The tiers a plan comparison screen lists. `master` is absent on purpose: it is the
 *  operator tier, not something anyone is offered. */
export const OFFERED_PLANS = ['free', 'basic', 'pro', 'max'] as const

/** One grant of credits. Consumption walks lots by expiry ascending with the non-expiring
 *  ones last, which is the order they are rendered in. */
export interface CreditLot {
  kind: 'monthly' | 'bonus'
  granted: number
  remaining: number
  /** RFC3339, or empty for a grant that does not expire. */
  expiresAt: string
}

/** What the account may spend. Credits are the product's own unit, so there is no currency
 *  here to format — only integers. */
export interface CreditBalance {
  credits: number
  /** The operator tier, which is never refused for balance: it shows no meter. */
  unlimited: boolean
  lots: CreditLot[]
  /** RFC3339; the instant the next monthly grant opens, computed by the server. */
  renewsAt: string
  /** What this tier is granted each month, so a meter has something to fill against. */
  monthlyGrant: number
}

/** One rung as the comparison screen lists it. Both figures come from the server: the grant
 *  and the price it was sized against are one product decision, and a client that carried
 *  its own copy of either would eventually disagree with the ladder it is describing. */
export interface PlanOffer {
  plan: PlanName | undefined
  monthlyCredits: number
  /** Whole US cents; zero for the free tier. Nothing here charges anyone. */
  priceUsdCents: number
}

export interface MyPlan {
  plan: PlanName | undefined
  balance: CreditBalance
  offers: PlanOffer[]
}

/** How many posts a balance covers at a given per-post estimate. It is deliberately a
 *  floor: telling someone they have "about 3" when the third would be refused is worse
 *  than telling them 2. */
export function postsAffordable(credits: number, perPost: number): number {
  if (perPost <= 0) return 0
  return Math.floor(credits / perPost)
}

/** One account as the operator screen sees it. */
export interface PlanAccount {
  id: string
  plan: PlanName | undefined
  createdAt: string
}
