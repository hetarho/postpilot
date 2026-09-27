import { expect, it } from 'vitest'
import { resumedJobStep } from './useEditorJob'

// POST-58: a job this editor did not start is owned by the step its kind belongs to — a storyline
// job by 글 생성 until the post holds a storyline, and by 글 다듬기 after (다시 만들기).
it('gives a resumed job to the step its kind belongs to', () => {
  expect(resumedJobStep('generate', false)).toBe('generate')
  expect(resumedJobStep('model_experiment', true)).toBe('generate')
  expect(resumedJobStep('revise', false)).toBe('refine')
  expect(resumedJobStep('revise_storyline', true)).toBe('refine')
  expect(resumedJobStep('storyline', false)).toBe('generate')
  expect(resumedJobStep('storyline', true)).toBe('refine')
  expect(resumedJobStep(undefined, false)).toBe('generate')
})
