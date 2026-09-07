import { describe, expect, it } from 'vitest'
import { postCostMilli, postsPerGrant, type EstimatorCombo } from './types'

/** The rates `plan_test.go` pins: a $0.30/$2.50 observer and a $1.00/$10.00 writer. */
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
