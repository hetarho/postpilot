import type { PlanOffer } from '@/entities/plan'

/** The public ladder, as static code-owned copy (MKT-5).
 *
 *  It mirrors the grant table in `backend/internal/plan` and is deliberately NOT a GetMyPlan
 *  read: a visitor with no account has no plan to read, and the ladder is a product fact rather
 *  than this visitor's state. The same numbers are repeated in the page test on purpose — that
 *  duplication is the drift alarm, so changing the ladder means changing both here in the same
 *  change. `pro` carries the code-owned recommended mark `/plans` shows (MKT-15); the post
 *  estimate does not travel here because it needs the operator's priced combos. */
export const PUBLIC_LADDER: readonly PlanOffer[] = [
  {
    plan: 'free',
    monthlyKrw: 0,
    annualKrw: 0,
    dailyCredits: 0,
    monthlyBonus: 0,
    modelCeiling: 'none',
    monthlyServerExports: 0,
    recommended: false,
  },
  {
    plan: 'light',
    monthlyKrw: 1900,
    annualKrw: 19000,
    dailyCredits: 15,
    monthlyBonus: 290,
    modelCeiling: 'value',
    monthlyServerExports: 2,
    recommended: false,
  },
  {
    plan: 'basic',
    monthlyKrw: 4900,
    annualKrw: 49000,
    dailyCredits: 45,
    monthlyBonus: 510,
    modelCeiling: 'balanced',
    monthlyServerExports: 6,
    recommended: false,
  },
  {
    plan: 'pro',
    monthlyKrw: 9900,
    annualKrw: 99000,
    dailyCredits: 85,
    monthlyBonus: 1070,
    modelCeiling: 'premium',
    monthlyServerExports: 15,
    recommended: true,
  },
  {
    plan: 'max',
    monthlyKrw: 29900,
    annualKrw: 299000,
    dailyCredits: 235,
    monthlyBonus: 3170,
    modelCeiling: 'top',
    monthlyServerExports: 60,
    recommended: false,
  },
]
