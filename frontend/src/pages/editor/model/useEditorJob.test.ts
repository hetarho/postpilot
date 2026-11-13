import { expect, it } from 'vitest'
import { resumedJobStep } from './useEditorJob'

// POST-58: a job this editor did not start is owned by the step its result lands on, where the
// control its retry calls is mounted.
it('gives a resumed job to the step its kind belongs to', () => {
  expect(resumedJobStep('observe')).toBe('observe')
  expect(resumedJobStep('storyline')).toBe('storyline')
  expect(resumedJobStep('revise_storyline')).toBe('storyline')
  expect(resumedJobStep('generate')).toBe('refine')
  expect(resumedJobStep('revise')).toBe('refine')
  expect(resumedJobStep('model_experiment')).toBe('generate')
  expect(resumedJobStep(undefined)).toBe('generate')
})
