import { expect, it, vi } from 'vitest'
import { EDITOR_HANDOFF_TTL_MS } from '@/shared/config'
import {
  activeGuideSteps,
  initialGuideStep,
  nextGuideStep,
  previousGuideStep,
  stashGuideStep,
  peekGuideStep,
  takeGuideStep,
  stashGuidelinesRequest,
  takeGuidelinesRequest,
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

it('hands a guideline opening only to its owner and minted post, once', () => {
  stashGuidelinesRequest('alice', 'saved')
  expect(takeGuidelinesRequest('bob', 'saved')).toBe(false)
  expect(takeGuidelinesRequest('alice', 'other')).toBe(false)
  expect(takeGuidelinesRequest('alice', 'saved')).toBe(true)
  expect(takeGuidelinesRequest('alice', 'saved')).toBe(false)
})

it('expires a guideline opening whose navigation was never completed', () => {
  vi.useFakeTimers()
  try {
    stashGuidelinesRequest('alice', 'old')
    vi.advanceTimersByTime(EDITOR_HANDOFF_TTL_MS + 1)
    expect(takeGuidelinesRequest('alice', 'old')).toBe(false)
  } finally {
    vi.useRealTimers()
  }
})
it('consumes a minted step once, including the early guide and late closing note', () => {
  for (const step of ['requirements', 'story', 'template'] as const) {
    stashGuideStep('minted', step)
    expect(peekGuideStep('minted')).toBe(step)
    expect(peekGuideStep('minted')).toBe(step)
    expect(takeGuideStep('minted')).toBe(step)
    expect(takeGuideStep('minted')).toBeUndefined()
  }
})
it('opens returning drafts with material on the summary', () => {
  const post = { memo: '', images: [], videos: [], activeJob: undefined, pendingExperimentId: '' }
  expect(initialGuideStep(post)).toBe('kind')
  expect(initialGuideStep({ ...post, memo: 'kept' })).toBe('review')
})
