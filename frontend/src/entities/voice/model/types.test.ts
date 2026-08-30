import { describe, expect, it } from 'vitest'
import { defaultVoice, sortVoices, voiceRefLabel } from './types'

describe('voiceRefLabel', () => {
  it('names an active voice plainly and a deleted one as a tombstone', () => {
    expect(voiceRefLabel({ name: '리뷰', deleted: false })).toBe('리뷰')
    expect(voiceRefLabel({ name: '리뷰', deleted: true })).toBe('삭제된 말투 · 리뷰')
  })
})

describe('sortVoices', () => {
  it('mirrors the directory order: active first, the default first among them, then by name', () => {
    const sorted = sortVoices([
      { id: 'c', name: '여행', isDefault: false, deleted: true },
      { id: 'b', name: '리뷰', isDefault: false, deleted: false },
      { id: 'a', name: '일기', isDefault: true, deleted: false },
      { id: 'd', name: '가이드', isDefault: false, deleted: false },
    ])
    expect(sorted.map((voice) => voice.id)).toEqual(['a', 'd', 'b', 'c'])
  })
})

describe('defaultVoice', () => {
  // A deleted voice can carry a stale flag in a fixture; only an active default counts.
  it('ignores a deleted voice even if it is flagged default', () => {
    expect(
      defaultVoice([
        { isDefault: true, deleted: true },
        { isDefault: false, deleted: false },
      ]),
    ).toBeUndefined()
  })
})
