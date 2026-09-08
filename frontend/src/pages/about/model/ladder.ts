import type { PlanOffer } from '@/entities/plan'

/** The public ladder, as static code-owned copy (MARKETING-5).
 *
 *  It mirrors the grant table in `backend/internal/plan` and is deliberately NOT a GetMyPlan
 *  read: a visitor with no account has no plan to read, and the ladder is a product fact rather
 *  than this visitor's state. The same numbers are repeated in the page test on purpose — that
 *  duplication is the drift alarm, so changing the ladder means changing both here in the same
 *  change. `pro` carries the code-owned recommended mark `/plans` shows (MARKETING-15); the post
 *  estimate does not travel here because it needs the operator's priced combos. */
export const PUBLIC_LADDER: readonly PlanOffer[] = [
  { plan: 'free', monthlyCredits: 50, priceUsdCents: 0, recommended: false },
  { plan: 'basic', monthlyCredits: 220, priceUsdCents: 200, recommended: false },
  { plan: 'pro', monthlyCredits: 575, priceUsdCents: 500, recommended: true },
  { plan: 'max', monthlyCredits: 1200, priceUsdCents: 1000, recommended: false },
]
