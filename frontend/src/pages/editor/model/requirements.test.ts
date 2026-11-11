import { describe, expect, it } from 'vitest'
import { EMPTY_GUIDE, ADVERTISER_KEYWORD_MAX } from '@/entities/advertiser-guide'
import {
  acceptRequirement,
  draftKeywordCoverage,
  editSuggestion,
  suggestRequirements,
} from './requirements'

describe('local guide suggestions', () => {
  it('spots Korean and English requirements without changing the guide', () => {
    const guide = {
      ...EMPTY_GUIDE,
      text: '필수 키워드: 성수 카페, 조용한 공간\n공백 제외 1,500자 이상, 사진 12장 이상\n#성수맛집 #카페',
    }
    const before = structuredClone(guide)
    expect(suggestRequirements(guide.text)).toEqual(
      expect.arrayContaining([
        { id: 'minChars:1500', kind: 'minChars', value: 1500 },
        { id: 'minPhotos:12', kind: 'minPhotos', value: 12 },
        { id: 'keywords:성수카페', kind: 'keywords', value: '성수 카페' },
        { id: 'hashtags:성수맛집', kind: 'hashtags', value: '성수맛집' },
      ]),
    )
    expect(guide).toEqual(before)
    expect(
      suggestRequirements(
        'Minimum 1,000 characters. At least 8 photos. Keywords: cozy cafe, good coffee',
      ),
    ).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ kind: 'minChars', value: 1000 }),
        expect.objectContaining({ kind: 'minPhotos', value: 8 }),
        expect.objectContaining({ kind: 'keywords', value: 'good coffee' }),
      ]),
    )
  })
  it('ignores ordinary numbers, maximums, out-of-range values and duplicates', () => {
    expect(
      suggestRequirements('2026년 11월 방문. 사진 50장 이상. 30자 이상. 사진 5장 이하.'),
    ).toEqual([])
    expect(suggestRequirements('#Brunch #brunch #카페\n키워드: 성수 카페, 성수카페')).toHaveLength(3)
    expect(suggestRequirements('')).toEqual([])
    expect(editSuggestion('minPhotos', '31')).toBeUndefined()
    expect(editSuggestion('keywords', 'x'.repeat(41))).toBeUndefined()
    expect(editSuggestion('hashtags', '#two words')).toBeUndefined()
  })
  it('accepts edited candidates only on confirmation, respecting limits and keeping unrelated values', () => {
    const candidate = editSuggestion('minPhotos', '7')!
    expect(acceptRequirement(EMPTY_GUIDE, candidate)).toEqual({ ...EMPTY_GUIDE, minPhotos: 7 })
    const keyword = editSuggestion('keywords', '성수 카페')!
    const full = {
      ...EMPTY_GUIDE,
      keywords: Array.from({ length: ADVERTISER_KEYWORD_MAX }, (_, i) => String(i)),
    }
    expect(acceptRequirement(full, keyword)).toBe(full)
    const guide = { ...EMPTY_GUIDE, text: 'keep', keywords: ['성수카페'] }
    expect(acceptRequirement(guide, keyword)).toBe(guide)
  })
  it('reads keyword coverage from enabled answers, title and closing notes only', () => {
    expect(
      draftKeywordCoverage(['성수 카페', 'COFFEE', '숨김', '합쳐짐'], {
        title: '',
        memo: 'coffee',
        answers: [
          { text: '성수카페', enabled: true },
          { text: '숨김', enabled: false },
          { text: '합쳐', enabled: true },
          { text: '짐', enabled: true },
        ],
      }).map((item) => item.present),
    ).toEqual([true, true, false, false])
  })
})
