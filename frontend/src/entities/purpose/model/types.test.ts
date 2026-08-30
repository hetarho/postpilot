import { describe, expect, it } from 'vitest'
import {
  PURPOSE_LIMITS,
  canSavePurpose,
  detachWarning,
  purposeChars,
  remainingChars,
} from './types'

describe('purpose field rules', () => {
  // The backend counts Unicode scalar values, so the client must too — otherwise the counter
  // and the refusal disagree on exactly the characters people actually type.
  it('counts scalar values, not UTF-16 units, and ignores surrounding space', () => {
    expect(purposeChars('  정보성 리뷰  ')).toBe(6)
    expect(purposeChars('가나다')).toBe(3)
    // An emoji outside the BMP is one character here and one on the server; String.length is 2.
    expect(purposeChars('🍜')).toBe(1)
    expect(remainingChars('가나다', 10)).toBe(7)
    // Negative rather than clamped, so an over-long paste can say how much to cut.
    expect(remainingChars('가'.repeat(12), 10)).toBe(-2)
  })

  it('requires a name and instructions, allows an empty description, and bounds all three', () => {
    const ok = { name: '리뷰', description: '', instructions: '지침' }
    expect(canSavePurpose(ok)).toBe(true)
    expect(canSavePurpose({ ...ok, name: '   ' })).toBe(false)
    expect(canSavePurpose({ ...ok, instructions: '  ' })).toBe(false)
    expect(canSavePurpose({ ...ok, name: '가'.repeat(PURPOSE_LIMITS.name) })).toBe(true)
    expect(canSavePurpose({ ...ok, name: '가'.repeat(PURPOSE_LIMITS.name + 1) })).toBe(false)
    expect(
      canSavePurpose({ ...ok, description: '가'.repeat(PURPOSE_LIMITS.description + 1) }),
    ).toBe(false)
    expect(
      canSavePurpose({ ...ok, instructions: '가'.repeat(PURPOSE_LIMITS.instructions + 1) }),
    ).toBe(false)
  })

  // The delete detaches; it never removes a post or its content.
  it('states the detach count and says plainly when there is nothing to detach', () => {
    expect(detachWarning(3)).toContain('3개의 글에서 용도가 해제됩니다')
    expect(detachWarning(3)).toContain('글과 본문은 그대로 남아요')
    expect(detachWarning(0)).toBe('이 용도를 쓰는 글이 없어요.')
  })
})
