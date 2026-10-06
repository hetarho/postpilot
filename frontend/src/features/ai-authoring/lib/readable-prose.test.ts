import { expect, it } from 'vitest'
import { readableAuthoringProse } from './readable-prose'

it('keeps friendly prose and bracketed notes while withholding compiled XML and JSON', () => {
  expect(readableAuthoringProse('조금 더 짧게 다듬었어요.')).toBe(true)
  expect(readableAuthoringProse('[도입] 오늘 느낀 점을 먼저 써 주세요.')).toBe(true)
  expect(readableAuthoringProse('<template><write>본문</write></template>')).toBe(false)
  expect(readableAuthoringProse('{"body":"compiled"}')).toBe(false)
  expect(readableAuthoringProse('[{"name":"draft"}]')).toBe(false)
})
