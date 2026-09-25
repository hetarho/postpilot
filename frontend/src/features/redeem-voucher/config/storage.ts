/** Where a gift link waits while its visitor signs in or signs up. `localStorage` rather than
 *  `sessionStorage`: the verification mail opens its link in a new tab. */
export const PENDING_GIFT_KEY = 'postpilot.pendingGift'

/** A pending gift older than this is dropped: it matches the verification link's own life,
 *  the longest detour the hand-back has to survive. */
export const PENDING_GIFT_TTL_MS = 24 * 60 * 60 * 1000
