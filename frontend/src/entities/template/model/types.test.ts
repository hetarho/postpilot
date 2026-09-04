import { describe, expect, it } from 'vitest'
import {
  TEMPLATE_LIMITS,
  canSaveTemplate,
  detachWarning,
  templateChars,
  remainingChars,
} from './types'

describe('template field rules', () => {
  // The backend counts Unicode scalar values, so the client must too — otherwise the counter
  // and the refusal disagree on exactly the characters people actually type.
  it('counts scalar values, not UTF-16 units, and ignores surrounding space', () => {
    expect(templateChars('  정보성 리뷰  ')).toBe(6)
    expect(templateChars('가나다')).toBe(3)
    // An emoji outside the BMP is one character here and one on the server; String.length is 2.
    expect(templateChars('🍜')).toBe(1)
    expect(remainingChars('가나다', 10)).toBe(7)
    // Negative rather than clamped, so an over-long paste can say how much to cut.
    expect(remainingChars('가'.repeat(12), 10)).toBe(-2)
  })

  it('requires a name and body, allows an empty description, and bounds all three', () => {
    const ok = { name: '리뷰', description: '', body: '지침' }
    expect(canSaveTemplate(ok)).toBe(true)
    expect(canSaveTemplate({ ...ok, name: '   ' })).toBe(false)
    expect(canSaveTemplate({ ...ok, body: '  ' })).toBe(false)
    expect(canSaveTemplate({ ...ok, name: '가'.repeat(TEMPLATE_LIMITS.name) })).toBe(true)
    expect(canSaveTemplate({ ...ok, name: '가'.repeat(TEMPLATE_LIMITS.name + 1) })).toBe(false)
    expect(
      canSaveTemplate({ ...ok, description: '가'.repeat(TEMPLATE_LIMITS.description + 1) }),
    ).toBe(false)
    expect(canSaveTemplate({ ...ok, body: '가'.repeat(TEMPLATE_LIMITS.body + 1) })).toBe(false)
  })

  // The delete detaches; it never removes a post or its content.
  it('states the detach count and says plainly when there is nothing to detach', () => {
    expect(detachWarning(3)).toContain('3개의 글에서 템플릿이 해제됩니다')
    expect(detachWarning(3)).toContain('글과 본문은 그대로 남아요')
    expect(detachWarning(0)).toBe('이 템플릿을 쓰는 글이 없어요.')
  })
})
