import { canEstimate, clipCostMilli, clipsPerGrant, illustrativeMonthlyCredits } from './types'
import { describe, expect, it } from 'vitest'
import { postsPerFigure } from './types'

describe('postsPerFigure', () => {
  it("floors what a grant covers at a level's per-post credits", () => {
    expect(postsPerFigure(220, 11)).toBe(20)
    expect(postsPerFigure(575, 11)).toBe(52)
  })

  // Zero is what a screen must state in words rather than render as "about 0 posts", and a
  // figure of nothing covers nothing rather than dividing by zero.
  it('answers zero when a grant cannot cover one post or the figure is empty', () => {
    expect(postsPerFigure(3, 11)).toBe(0)
    expect(postsPerFigure(100, 0)).toBe(0)
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
