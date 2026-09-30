import { canEstimate, clipCostMilli, clipsPerGrant, illustrativeMonthlyCredits } from './types'
import { describe, expect, it } from 'vitest'
import { postCostMilli, postsPerGrant, type EstimatorCombo } from './types'

/** Arbitrary published rates: the client multiplies without owning token assumptions. */
const RATES: EstimatorCombo = {
  combo: 'balanced',
  observeLabel: 'vendor/eyes',
  writeLabel: 'vendor/pen',
  perPhotoMilli: 723,
  perVideoMilli: 1100,
  perThousandCharsMilli: 3600,
  perPostBaseMilli: 3800,
}

describe('postCostMilli', () => {
  it('adds the base to what each unit of work costs', () => {
    // 3800 + 5x723 + 1x1100 + 1x3600
    expect(postCostMilli(RATES, { chars: 1000, photos: 5, videos: 1 })).toBe(12115)
    // A post with nothing attached still pays for its write call.
    expect(postCostMilli(RATES, { chars: 0, photos: 0, videos: 0 })).toBe(3800)
  })

  // A partial thousand still costs a whole call's output ceiling, and rounding up also keeps
  // the figure monotonic as a slider moves.
  it('rounds characters up to the next thousand', () => {
    expect(postCostMilli(RATES, { chars: 1, photos: 0, videos: 0 })).toBe(3800 + 3600)
    expect(postCostMilli(RATES, { chars: 1001, photos: 0, videos: 0 })).toBe(3800 + 2 * 3600)
  })

  it('reads a negative input as none rather than as a discount', () => {
    expect(postCostMilli(RATES, { chars: -500, photos: -3, videos: -1 })).toBe(3800)
  })
})

describe('postsPerGrant', () => {
  it('floors what a grant covers', () => {
    // 11,015 milli-credits a post: 220 credits covers 19, 575 covers 52.
    const shape = { chars: 1000, photos: 5, videos: 0 }
    expect(postsPerGrant(220, RATES, shape)).toBe(19)
    expect(postsPerGrant(575, RATES, shape)).toBe(52)
  })

  // Zero is what a screen must state in words rather than render as "about 0 posts".
  it('answers zero when the grant cannot cover one post', () => {
    expect(postsPerGrant(3, RATES, { chars: 1000, photos: 5, videos: 0 })).toBe(0)
  })
})

describe('monthly illustrations and rights', () => {
  const offer = {
    plan: 'basic',
    monthlyKrw: 4900,
    annualKrw: 49000,
    dailyCredits: 45,
    monthlyBonus: 510,
    modelCeiling: 'balanced',
    monthlyServerExports: 6,
    recommended: false,
  } as const
  it('uses assumed daily grants plus the bonus', () => {
    expect(illustrativeMonthlyCredits(offer)).toBe(1860)
    expect(illustrativeMonthlyCredits(offer, 28)).toBe(1770)
  })
  it('locks levels above the tier and has no paid estimate for free', () => {
    expect(canEstimate(offer, 'value')).toBe(true)
    expect(canEstimate(offer, 'balanced')).toBe(true)
    expect(canEstimate(offer, 'premium')).toBe(false)
    expect(canEstimate({ ...offer, plan: 'free' }, 'value')).toBe(false)
  })
})

describe('clip estimates', () => {
  const rates = { perSourceMilli: 8750, perOutputSecondMilli: 360, perClipBaseMilli: 11200 }
  it('prices original count independently from finished seconds and floors the result', () => {
    expect(clipCostMilli(rates, { sources: 3, seconds: 30 })).toBe(48250)
    expect(clipCostMilli(rates, { sources: 4, seconds: 30 })).toBe(57000)
    expect(clipCostMilli(rates, { sources: 3, seconds: 60 })).toBe(59050)
    expect(clipsPerGrant(330, rates, { sources: 3, seconds: 30 })).toBe(6)
    expect(clipsPerGrant(50, rates, { sources: 3, seconds: 60 })).toBe(0)
  })
  it('never refunds negative inputs or divides by zero', () => {
    expect(clipCostMilli(rates, { sources: -2, seconds: -30 })).toBe(11200)
    expect(
      clipsPerGrant(
        50,
        { perSourceMilli: 0, perOutputSecondMilli: 0, perClipBaseMilli: 0 },
        { sources: 3, seconds: 30 },
      ),
    ).toBe(0)
  })
})
