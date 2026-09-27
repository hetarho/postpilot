import { expect, it } from 'vitest'
import { progressLabel } from './types'

// GEN-68: a storyline job names its one stage.
it('names the storyline stage', () => {
  expect(progressLabel({ kind: 'storyline', stage: 'storyline' })).toBe('스토리라인 작성 중')
  expect(progressLabel({ kind: 'revise_storyline', stage: 'storyline' })).toBe('스토리라인 작성 중')
  expect(progressLabel({ kind: 'storyline', stage: 'observe' })).toBe('사진 관찰 중')
})

// CLIP-177: the clip storyline call names its stage the way the post's does.
it('names the clip storyline stage', () => {
  expect(progressLabel({ kind: 'storyline_clip', stage: 'storyline' })).toBe('스토리라인 작성 중')
  expect(progressLabel({ kind: 'revise_storyline_clip', stage: 'storyline' })).toBe(
    '스토리라인 작성 중',
  )
})
