import { expect, it } from 'vitest'
import { draftStep, draftStepStart } from './useDraftSteps'

it('starts on the step the post’s status belongs to, unknown statuses included', () => {
  expect(draftStepStart('draft').step).toBe('generate')
  expect(draftStepStart('review').step).toBe('refine')
  expect(draftStepStart('finalized').step).toBe('finish')
  expect(draftStepStart('').step).toBe('generate')
  expect(draftStepStart('archived-in-some-later-plan').step).toBe('generate')
})

it('keeps a picked step until the post’s status moves', () => {
  let state = draftStepStart('draft')
  state = draftStep(state, { type: 'select', step: 'finish' })
  expect(state.step).toBe('finish')
  // The same status again is not a transition: a re-render must not yank the reader back.
  expect(draftStep(state, { type: 'status', status: 'draft' })).toBe(state)
  // Picking the step already shown is not a transition either.
  expect(draftStep(state, { type: 'select', step: 'finish' })).toBe(state)
  // A generation that finished moves the post to `review`, and the reader with it.
  state = draftStep(state, { type: 'status', status: 'review' })
  expect(state.step).toBe('refine')
})

it('follows every later status change, including one back to an earlier step', () => {
  let state = draftStepStart('review')
  state = draftStep(state, { type: 'status', status: 'finalized' })
  expect(state.step).toBe('finish')
  state = draftStep(state, { type: 'select', step: 'refine' })
  state = draftStep(state, { type: 'status', status: 'draft' })
  expect(state).toEqual({ step: 'generate', followed: 'draft' })
})
