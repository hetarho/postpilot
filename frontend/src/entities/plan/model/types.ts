import type { PostCreditsBasisName } from '@/shared/api'
import i18next from 'i18next'
import { PAID_LEVELS, type LevelName } from '@/entities/model-catalog/@x/plan'

/** The ladder, in order. */
export const PLANS = ['free', 'light', 'basic', 'pro', 'max', 'master'] as const

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
export const OFFERED_PLANS = ['free', 'light', 'basic', 'pro', 'max'] as const

/** Current offer authority, independent of legacy positive export balances. */
export function hasServerExportAccess(value: MyPlan | undefined): boolean {
  return (
    !!value?.plan &&
    (value.plan === 'master' ||
      value.offers.some((offer) => offer.plan === value.plan && offer.monthlyServerExports > 0))
  )
}

/** One grant of credits. Consumption spends every expiring lot first by expiry — monthly,
 *  voucher and an expiring bonus alike — then never-expiring bonus, then purchased (QUOTA-12),
 *  which is the order the server sends and the order they are rendered in. */
export interface CreditLot {
  kind: 'monthly' | 'bonus' | 'purchased' | 'voucher' | 'daily' | 'compensation'
  granted: number
  remaining: number
  /** RFC3339, or empty for a grant that does not expire. */
  expiresAt: string
  issuanceCause: string
}

/** What the account may spend. Credits are the product's own unit, so there is no currency
 *  here to format — only integers. */
export interface CreditBalance {
  credits: number
  /** The operator tier, which is never refused for balance: it shows no meter. */
  unlimited: boolean
  lots: CreditLot[]
  /** RFC3339; the next paid credit reset computed by the server. */
  renewsAt: string
  /** What this tier is granted each month, so a meter has something to fill against. */
  dailyGrant: number
  monthlyBonus: number
  dailyResetsAt: string
  bonusResetsAt: string
}

/** One published rung. The server owns its price, benefit clocks, model ceiling and exports. */
export interface PlanOffer {
  plan: PlanName | undefined
  monthlyKrw: number
  annualKrw: number
  dailyCredits: number
  monthlyBonus: number
  modelCeiling: 'none' | 'value' | 'balanced' | 'premium' | 'top'
  monthlyServerExports: number
  /** The one rung the comparison screen marks. The server decides which. */
  recommended: boolean
}

/** The four model levels a post estimate can be quoted at. The operator assigns models
 *  carrying the same per-purpose level behind each one (QUOTA-39). */
export const ESTIMATOR_COMBOS = PAID_LEVELS

export type EstimatorComboName = Exclude<LevelName, 'free'>

/** One combo's unit costs in MILLI-credits — thousandths, so the arithmetic stays in
 *  integers. The server derives them from what its two models really charge (QUOTA-40) and
 *  the client is only allowed to multiply. */
/** The rate paid AI work is priced at, as only /admin's 비용·환율 tab reads it (QUOTA-65).
 *  `undefined` rate with `unavailable` means no eligible rate: paid AI work cannot start. */
export interface ExchangeRate {
  rate?: {
    source: string
    publicationDate: string
    /** KRW per USD, with the four decimals the wire carries as `_e4` already divided out. */
    reference: number
    applied: number
    temporary: boolean
  }
  unavailable: boolean
}

export interface EstimatorCombo {
  combo: EstimatorComboName
  /** One post with photos on this level's pair, in credits, and where the figure came from
   *  (QUOTA-64). Absent when either stage has no figure. */
  postCredits?: { credits: number; basis: PostCreditsBasisName }
  clipRates?: ClipEstimatorRates
}

export interface ClipEstimatorRates {
  perSourceMilli: number
  perOutputSecondMilli: number
  perClipBaseMilli: number
}

export interface MyPlan {
  plan: PlanName | undefined
  balance: CreditBalance
  offers: PlanOffer[]
  creditPacks: { id: string; priceKrw: number; credits: number }[]
  /** The combos the operator has assigned and the server could price. Empty means a
   *  comparison shows grants and prices with no post estimate. */
  estimatorCombos: EstimatorCombo[]
  clipSourceSeconds: number
  fxUnavailable: boolean
  serverExportWindow?: {
    coverageId: string
    startsAt: string
    endsAt: string
    allowance: number
    used: number
    reserved: number
    remaining: number
  }
}

/** How many posts a balance covers at a given per-post estimate. It is deliberately a
 *  floor: telling someone they have "about 3" when the third would be refused is worse
 *  than telling them 2. */
export function postsAffordable(credits: number, perPost: number): number {
  if (perPost <= 0) return 0
  return Math.floor(credits / perPost)
}

/** How many posts an illustrative credit amount covers at a level's per-post figure. A floor,
 *  like postsAffordable; zero means it covers less than one. */
export function postsPerFigure(illustrativeCredits: number, perPostCredits: number): number {
  if (perPostCredits <= 0) return 0
  return Math.floor(illustrativeCredits / perPostCredits)
}

/** Clip conditions describe original count and FINISHED duration. Original duration is
 * assumed by the server and is already included in the per-source rate. */
export function clipCostMilli(
  rates: ClipEstimatorRates,
  input: { sources: number; seconds: number },
): number {
  return (
    rates.perClipBaseMilli +
    Math.max(0, input.sources) * rates.perSourceMilli +
    Math.max(0, input.seconds) * rates.perOutputSecondMilli
  )
}

export function clipsPerGrant(
  illustrativeCredits: number,
  rates: ClipEstimatorRates,
  input: { sources: number; seconds: number },
): number {
  const cost = clipCostMilli(rates, input)
  return cost > 0 ? Math.floor((illustrativeCredits * 1000) / cost) : 0
}

/** Illustration only: daily grants expire daily and are not available upfront. */
export function illustrativeMonthlyCredits(offer: PlanOffer, assumedDays = 30): number {
  return offer.dailyCredits * assumedDays + offer.monthlyBonus
}

/** One account as the operator screen sees it. */
export interface PlanAccount {
  id: string
  plan: PlanName | undefined
  createdAt: string
}
