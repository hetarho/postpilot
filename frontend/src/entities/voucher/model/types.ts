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
