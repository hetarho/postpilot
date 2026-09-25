/** Where a voucher stands, derived by the server at read time (GIFT). `unknown` is a state
 *  this client was never told about, rendered like one that cannot be redeemed. */
export type VoucherState = 'redeemable' | 'redeemed' | 'expired' | 'revoked' | 'unknown'

/** What the public gift page may show about a voucher (GIFT-8): never who issued it, what
 *  was paid, or who redeemed it. Instants are RFC3339. */
export interface GiftView {
  credits: number
  validityDays: number
  message: string
  state: VoucherState
  linkExpiresAt: string
}

/** What a redemption handed the account. */
export interface Redemption {
  credits: number
  creditsExpireAt: string
}

/** What a sold voucher was paid, as the operator entered it (GIFT-4). */
export interface VoucherSale {
  amountKrw: number
  payerName: string
}

/** A voucher as the operator's list shows it (GIFT-14). `token` is present only while the
 *  link can still be redeemed; instants are RFC3339 and empty when absent. */
export interface Voucher {
  id: string
  credits: number
  validityDays: number
  sale?: VoucherSale
  message: string
  state: VoucherState
  issuedAt: string
  linkExpiresAt: string
  redeemedBy: string
  redeemedAt: string
  remainingCredits: number
  creditsExpireAt: string
  token: string
}

/** One issue shortcut: a paid rung's monthly grant for a fixed number of days (GIFT-3). */
export interface VoucherPreset {
  plan: 'basic' | 'pro' | 'max' | 'other'
  credits: number
  validityDays: number
}

/** What the operator asks for. A given voucher has no sale. */
export interface VoucherIssue {
  credits: number
  validityDays: number
  message: string
  sale?: VoucherSale
}

/** The product's bounds on an issue, mirrored from the server so the form can say what is
 *  wrong before it asks; the server still refuses anything outside them. */
export const VOUCHER_LIMITS = {
  maxCredits: 100_000,
  maxDays: 365,
  maxMessage: 80,
  maxPayer: 40,
  maxSaleKrw: 10_000_000,
} as const
