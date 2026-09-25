import { describe, expect, it } from 'vitest'
import { qualityValuesOf, type QualityReading } from './types'

const reading = (values: QualityReading['values']): QualityReading => ({
  metric: 'cross_post_phrases',
  verdict: 'over_band',
  minimum: 3,
  publishedCount: 5,
  ruleText: '',
  values,
})

describe('qualityValuesOf', () => {
  it('returns the values when they are the named metric’s', () => {
    const values = { metric: 'cross_post_phrases' as const, share: 0.15, shareWarnAbove: 0.1 }
    expect(qualityValuesOf(reading(values), 'cross_post_phrases')).toBe(values)
  })

  it('returns undefined for another metric’s values or for none', () => {
    const values = { metric: 'title_saturation' as const, share: 0.4, shareWarnAbove: 0.3 }
    expect(qualityValuesOf(reading(values), 'cross_post_phrases')).toBeUndefined()
    expect(qualityValuesOf(reading(undefined), 'cross_post_phrases')).toBeUndefined()
  })
})
