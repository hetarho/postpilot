import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { normalizeAppFailure } from '@/shared/api'
import { formatAppFailure } from './failure'

afterEach(() => initializeI18n('ko'))

// POST-13, POST-20: the two post refusals carry the numbers the reader needs — how many photo
// places remain, and the range a 목표 글자 수 missed — and say what to do, in both languages.
describe('post refusals', () => {
  it('says how many photo places name a detached photo and how to fix them', () => {
    const refusal = normalizeAppFailure({ reason: 'POST_PHOTO_MISSING', params: { count: '2' } })
    initializeI18n('ko')
    expect(formatAppFailure(refusal)).toContain('2곳')
    expect(formatAppFailure(refusal)).toContain('사진을 다시 올린')
    initializeI18n('en')
    expect(formatAppFailure(refusal, 'en')).toContain('2 photo places')
  })

  it('names the range a target length missed', () => {
    const refusal = normalizeAppFailure({
      reason: 'POST_TARGET_LENGTH_INVALID',
      params: { min: '100', max: '10000' },
    })
    initializeI18n('ko')
    expect(formatAppFailure(refusal)).toContain('100~10000자')
    initializeI18n('en')
    expect(formatAppFailure(refusal, 'en')).toContain('between 100 and 10000')
  })
})
