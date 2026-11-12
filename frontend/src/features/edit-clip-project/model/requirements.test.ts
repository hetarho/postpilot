import { expect, it } from 'vitest'
import { clipKeywordCoverage } from './requirements'

it('counts a keyword held by the title, the instruction or an answer, each on its own', () => {
  expect(
    clipKeywordCoverage(
      '성수 카페 브이로그',
      '라떼 위주로',
      ['#디저트 맛집', '합쳐'],
      ['성수카페', 'LATTE', '디저트맛집', '합쳐짐', '라떼'],
    ).map((item) => item.held),
  ).toEqual([true, false, true, false, true])
})
