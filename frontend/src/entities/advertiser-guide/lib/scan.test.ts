import { describe, expect, it } from 'vitest'
import { EMPTY_GUIDE } from '../model/types'
import { ADVERTISER_GUIDE_MAX_CHARS, ADVERTISER_KEYWORD_MAX } from '../config'
import { POST_GUIDE_ITEMS } from '../model/types'
import {
  acceptRequirement,
  appendHashtags,
  editSuggestion,
  guideHashtags,
  keywordCoverage,
  suggestRequirements,
} from './scan'

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
    expect(suggestRequirements('#Brunch #brunch #카페\n키워드: 성수 카페, 성수카페')).toHaveLength(
      3,
    )
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
  it('offers only the counted items its host asks for', () => {
    expect(
      suggestRequirements('사진 8장 이상, 1,500자 이상\n#성수 키워드: 카페', [
        'keywords',
        'hashtags',
      ])
        .map((item) => item.kind)
        .sort(),
    ).toEqual(['hashtags', 'keywords'])
  })
  it('holds a keyword only inside one text, ignoring case, spaces and a leading #', () => {
    expect(
      keywordCoverage(['#성수 카페', 'COFFEE', '합쳐짐'], ['성수카페', 'coffee', '합쳐', '짐']).map(
        (item) => item.held,
      ),
    ).toEqual([true, true, false])
  })
  // POST-118: a post's list offers no hashtag chip, while its other chips still appear.
  it('offers a post no hashtag chip', () => {
    expect(
      suggestRequirements('사진 8장 이상, 1,500자 이상\n#성수 키워드: 카페', POST_GUIDE_ITEMS)
        .map((item) => item.kind)
        .sort(),
    ).toEqual(['keywords', 'minChars', 'minPhotos'])
  })
})

describe('hashtags kept in the guide text', () => {
  it('finds each #hashtag once, in text order, ignoring case', () => {
    expect(guideHashtags('#성수맛집 꼭\n#Brunch, #brunch #카페_투어')).toEqual([
      '성수맛집',
      'Brunch',
      '카페_투어',
    ])
    expect(guideHashtags('해시태그 없음 # 띄어씀')).toEqual([])
  })
  it('appends the hashtags the text lacks as one last line', () => {
    expect(appendHashtags('주차 안내\n#성수 ', ['성수', '#카페', 'Latte', 'latte'])).toBe(
      '주차 안내\n#성수\n#카페 #Latte',
    )
    expect(appendHashtags('', ['성수'])).toBe('#성수')
    expect(appendHashtags('#성수', ['#성수'])).toBe('#성수')
  })
  it('stops before a token that would pass the guide limit', () => {
    const text = 'x'.repeat(ADVERTISER_GUIDE_MAX_CHARS - 6)
    const appended = appendHashtags(text, ['ab', 'cd'])
    expect(appended).toBe(`${text}\n#ab`)
    expect([...appended].length).toBeLessThanOrEqual(ADVERTISER_GUIDE_MAX_CHARS)
    expect(appendHashtags('x'.repeat(ADVERTISER_GUIDE_MAX_CHARS), ['ab'])).toBe(
      'x'.repeat(ADVERTISER_GUIDE_MAX_CHARS),
    )
  })
})
