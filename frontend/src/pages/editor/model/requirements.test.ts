import { describe, expect, it } from 'vitest'
import { draftKeywordCoverage } from './requirements'

describe('draft keyword coverage', () => {
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
