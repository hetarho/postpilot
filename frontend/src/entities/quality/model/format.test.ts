import { describe, expect, it } from 'vitest'
import { absentValueLabel, formatShare, formatShareOrAbsent } from './format'

describe('formatShareOrAbsent', () => {
  // QUAL-40: a value the server could not compute is not zero.
  it('says a missing share could not be measured, and formats any other', () => {
    expect(formatShareOrAbsent(undefined)).toBe(absentValueLabel())
    expect(formatShareOrAbsent(undefined)).toBe('측정할 수 없어요')
    expect(formatShareOrAbsent(0)).toBe(formatShare(0))
    expect(formatShareOrAbsent(0.425)).toBe(formatShare(0.425))
  })
})
