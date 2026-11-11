import { expect, it } from 'vitest'
import {
  activeGuideSteps,
  initialGuideStep,
  nextGuideStep,
  previousGuideStep,
  stashGuideStep,
  takeGuideStep,
} from './guide'

it('asks campaign context first, then photos before template and every writing input', () => {
  expect(activeGuideSteps()).toEqual([
    'kind',
    'requirements',
    'photos',
    'template',
    'details',
    'story',
    'review',
  ])
  expect(initialGuideStep(undefined)).toBe('kind')
  expect(nextGuideStep('kind')).toBe('requirements')
  expect(nextGuideStep('kind', false)).toBe('photos')
  expect(previousGuideStep('photos', false)).toBe('kind')
  expect(activeGuideSteps(false)).not.toContain('requirements')
  expect(nextGuideStep('story')).toBe('review')
  expect(nextGuideStep('review')).toBeUndefined()
})
it('consumes a minted step once, including the early guide and late closing note', () => {
  for (const step of ['requirements', 'story', 'template'] as const) {
    stashGuideStep('minted', step)
    expect(takeGuideStep('minted')).toBe(step)
    expect(takeGuideStep('minted')).toBeUndefined()
  }
})
it('opens returning drafts with material on the summary', () => {
  const post = { memo: '', images: [], videos: [], activeJob: undefined, pendingExperimentId: '' }
  expect(initialGuideStep(post)).toBe('kind')
  expect(initialGuideStep({ ...post, memo: 'kept' })).toBe('review')
})
