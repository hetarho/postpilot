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

// POST-46: no post job analyzes any more, so the editor's labels have no 문체 분석 중, while the
// voice page's own analysis keeps naming its stage.
it('names the analyze stage for a voice analysis alone', () => {
  expect(progressLabel({ kind: 'analyze_voice', stage: 'analyze' })).toBe('문체 분석 중')
  expect(progressLabel({ kind: 'revise', stage: 'analyze' })).toBe('생성 중')
  expect(progressLabel({ stage: 'analyze' })).toBe('생성 중')
})
