import { expect, it } from 'vitest'
import { draftStep, draftStepStart } from './useDraftSteps'

const facts = (status: string, hasObservations = false, hasStoryline = false) => ({
  status,
  hasObservations,
  hasStoryline,
})
const event = (status: string, hasObservations = false, hasStoryline = false) => ({
  type: 'facts' as const,
  ...facts(status, hasObservations, hasStoryline),
})

it('starts on the step the post’s facts belong to, unknown statuses included', () => {
  expect(draftStepStart(facts('draft')).step).toBe('generate')
  expect(draftStepStart(facts('draft', true)).step).toBe('observe')
  expect(draftStepStart(facts('draft', true, true)).step).toBe('storyline')
  expect(draftStepStart(facts('review')).step).toBe('refine')
  expect(draftStepStart(facts('finalized')).step).toBe('finish')
  expect(draftStepStart(facts('published')).step).toBe('finish')
  expect(draftStepStart(facts('')).step).toBe('generate')
  expect(draftStepStart(facts('archived-in-some-later-plan')).step).toBe('generate')
})

it('keeps a picked step until the post’s facts move', () => {
  let state = draftStepStart(facts('draft'))
  state = draftStep(state, { type: 'select', step: 'finish' })
  expect(state.step).toBe('finish')
  // The same facts again are not a transition: a re-render must not yank the reader back.
  expect(draftStep(state, event('draft'))).toBe(state)
  // Picking the step already shown is not a transition either.
  expect(draftStep(state, { type: 'select', step: 'finish' })).toBe(state)
  // A writing run that finished moves the post to `review`, and the reader with it.
  state = draftStep(state, event('review'))
  expect(state.step).toBe('refine')
})

it('follows every later status change, including one back to an earlier step', () => {
  let state = draftStepStart(facts('review'))
  state = draftStep(state, event('finalized'))
  expect(state.step).toBe('finish')
  state = draftStep(state, { type: 'select', step: 'refine' })
  state = draftStep(state, event('draft'))
  expect(state).toEqual({ step: 'generate', followed: 'draft' })
})

it('keeps the reader on 완성 when a finalized post is published', () => {
  let state = draftStepStart(facts('finalized'))
  state = draftStep(state, event('published'))
  expect(state).toEqual({ step: 'finish', followed: 'published' })
})

// An observation run and a storyline run finish without a status change (POST-44, POST-125).
it('hands a finished observation run to 관찰 and a finished storyline run to 스토리라인', () => {
  let state = draftStepStart(facts('draft'))
  expect(state.step).toBe('generate')
  state = draftStep(state, event('draft', true))
  expect(state.step).toBe('observe')
  // The same observations again are not a transition.
  expect(draftStep(state, event('draft', true))).toBe(state)
  state = draftStep(state, { type: 'select', step: 'generate' })
  state = draftStep(state, event('draft', true, true))
  expect(state.step).toBe('storyline')
  expect(draftStep(state, event('draft', true, true))).toBe(state)
  // The writing run that follows lands on 다듬기.
  state = draftStep(state, event('review', true, true))
  expect(state.step).toBe('refine')
})
