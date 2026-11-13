import { describe, expect, it } from 'vitest'
import { editorSteps, stepForPost } from './steps'

const facts = (status: string, hasObservations = false, hasStoryline = false) => ({
  status,
  hasObservations,
  hasStoryline,
})

describe('stepForPost (POST-44)', () => {
  it('maps each shipped status to its step', () => {
    expect(stepForPost(facts('draft'))).toBe('generate')
    expect(stepForPost(facts('review'))).toBe('refine')
    expect(stepForPost(facts('finalized'))).toBe('finish')
    // A published post has nothing left to write: it opens where its URL lives.
    expect(stepForPost(facts('published'))).toBe('finish')
  })

  it('falls back to the first step for an unknown status', () => {
    expect(stepForPost(facts('archived'))).toBe('generate')
    expect(stepForPost(facts(''))).toBe('generate')
  })

  it('opens a draft on the step its persisted material belongs to', () => {
    expect(stepForPost(facts('draft', true))).toBe('observe')
    expect(stepForPost(facts('draft', true, true))).toBe('storyline')
    expect(stepForPost(facts('draft', false, true))).toBe('storyline')
    // Past the draft, the status decides whatever the post still holds.
    expect(stepForPost(facts('review', true, true))).toBe('refine')
    expect(stepForPost(facts('finalized', true, true))).toBe('finish')
  })

  it('labels the five stations in lifecycle order by their short names', () => {
    expect(editorSteps().map((step) => step.value)).toEqual([
      'generate',
      'observe',
      'storyline',
      'refine',
      'finish',
    ])
    expect(editorSteps().map((step) => step.label)).toEqual([
      '생성',
      '관찰',
      '스토리라인',
      '다듬기',
      '완성',
    ])
  })
})
