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

/** One grant of credits. Consumption walks lots by kind first — monthly, then bonus, then
 *  purchased — and only then by expiry, which is the order they are rendered in. */
export interface CreditLot {
  kind: 'monthly' | 'bonus' | 'purchased'
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
  /** Whole US cents; zero for the free tier. What a card is charged is BILLING's. */
  priceUsdCents: number
  /** The one rung the comparison screen marks. The server decides which. */
  recommended: boolean
}

/** The four price tiers a post estimate can be quoted at. The operator assigns the models
 *  behind each one; a screen names the tier (QUOTA-39). */
export const ESTIMATOR_COMBOS = ['quality', 'balanced', 'value', 'cheapest'] as const

export type EstimatorComboName = (typeof ESTIMATOR_COMBOS)[number]

/** One combo's unit costs in MILLI-credits — thousandths, so the arithmetic stays in
 *  integers. The server derives them from what its two models really charge (QUOTA-40) and
 *  the client is only allowed to multiply. */
export interface EstimatorCombo {
  combo: EstimatorComboName
  /** The models behind the tier, for the operator's own screen only. */
  observeLabel: string
  writeLabel: string
  perPhotoMilli: number
  perVideoMilli: number
  perThousandCharsMilli: number
  perPostBaseMilli: number
}

export interface MyPlan {
  plan: PlanName | undefined
  balance: CreditBalance
  offers: PlanOffer[]
  /** The combos the operator has assigned and the server could price. Empty means a
   *  comparison shows grants and prices with no post estimate. */
  estimatorCombos: EstimatorCombo[]
}

/** How many posts a balance covers at a given per-post estimate. It is deliberately a
 *  floor: telling someone they have "about 3" when the third would be refused is worse
 *  than telling them 2. */
export function postsAffordable(credits: number, perPost: number): number {
  if (perPost <= 0) return 0
  return Math.floor(credits / perPost)
}

/** What one post of the given shape costs, in MILLI-credits, at one combo's rates.
 *
 *  This is the whole of the client's arithmetic (QUOTA-40): the server owns the charge
 *  formula and publishes rates, and the page multiplies. Characters round UP to the next
 *  thousand — a partial thousand still costs a whole call's output ceiling — which also
 *  keeps the figure monotonic as a slider moves. */
export function postCostMilli(
  rates: EstimatorCombo,
  input: { chars: number; photos: number; videos: number },
): number {
  const thousands = Math.max(0, Math.ceil(input.chars / 1000))
  return (
    rates.perPostBaseMilli +
    Math.max(0, input.photos) * rates.perPhotoMilli +
    Math.max(0, input.videos) * rates.perVideoMilli +
    thousands * rates.perThousandCharsMilli
  )
}

/** How many posts of that shape a monthly grant covers. Zero means the grant does not cover
 *  one, which a screen states rather than rendering as "about 0". */
export function postsPerGrant(
  monthlyCredits: number,
  rates: EstimatorCombo,
  input: { chars: number; photos: number; videos: number },
): number {
  const costMilli = postCostMilli(rates, input)
  if (costMilli <= 0) return 0
  return Math.floor((monthlyCredits * 1000) / costMilli)
}

/** One account as the operator screen sees it. */
export interface PlanAccount {
  id: string
  plan: PlanName | undefined
  createdAt: string
}
