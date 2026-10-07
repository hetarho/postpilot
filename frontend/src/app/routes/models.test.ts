import { isRedirect } from '@tanstack/react-router'
import { expect, it } from 'vitest'
import { writingTestHistorySearchSchema } from '@/pages/writing-tests'
import {
  aiModelsRoute,
  modelComparisonRoute,
  modelHistoryRoute,
  modelLeaderboardRoute,
  modelExperimentRoute,
  legacyModelComparisonSearchSchema,
  redirectLegacyModelComparison,
  redirectLegacyModelHistory,
} from './models'

function redirected(action: () => never) {
  try {
    action()
  } catch (error) {
    if (!isRedirect(error)) throw error
    return error.options
  }
  throw new Error('Expected an explicit alias redirect')
}

it.each(['observe', 'write', 'voice', 'analyze', undefined])(
  'moves the old %s comparison to one explicit two-entry test with retained source and voice',
  (stage) => {
    const search = legacyModelComparisonSearchSchema({
      stage,
      count: 5,
      sourcePost: 'owned-source',
      voiceId: 'owned-voice',
      window: 'month',
      scope: 'all',
      entry: '/settings?tab=models',
    })
    const result = redirected(() => redirectLegacyModelComparison(search))
    expect(legacyModelComparisonSearchSchema({ ...search })).toEqual(search)
    expect(result).toMatchObject({
      to: '/tests',
      replace: true,
      search: {
        factor: stage === 'voice' ? 'voice' : 'model',
        stage: stage === 'observe' ? 'observe' : 'write',
        count: 2,
        source: 'owned-source',
        voiceId: 'owned-voice',
        entry: '/settings?tab=models',
      },
    })
    expect(result.search).not.toHaveProperty('window')
    expect(result.search).not.toHaveProperty('scope')
  },
)

it.each(['sourcePost', 'sourcePostSlug', 'postSlug'])(
  'keeps the %s source alias without treating malformed values as usable settings',
  (key) => {
    expect(legacyModelComparisonSearchSchema({ [key]: 'source' }).source).toBe('source')
    expect(
      legacyModelComparisonSearchSchema({ [key]: ['source'], voiceId: {}, entry: '//foreign' }),
    ).toMatchObject({ source: undefined, voiceId: undefined, entry: undefined })
  },
)

it('replaces old history and leaderboard destinations with filtered private history and drops ranking scope', () => {
  const result = redirected(() =>
    redirectLegacyModelHistory(
      writingTestHistorySearchSchema({
        stage: 'voice',
        sourcePostSlug: 'source',
        voiceId: 'voice',
        window: 'week',
        scope: 'all',
      }),
    ),
  )
  expect(result).toMatchObject({
    to: '/tests/history',
    replace: true,
    search: { stage: 'voice', source: 'source', voiceId: 'voice' },
  })
  expect(result.search).not.toHaveProperty('scope')
  expect(result.search).not.toHaveProperty('window')
  expect(modelComparisonRoute.options.beforeLoad).toBeTypeOf('function')
  expect(modelHistoryRoute.options.beforeLoad).toBeTypeOf('function')
  expect(modelLeaderboardRoute.options.beforeLoad).toBeTypeOf('function')
})

it('keeps active model settings and paid experiment detail routes instead of redirecting their stored records', () => {
  expect('path' in aiModelsRoute.options && aiModelsRoute.options.path).toBe('/ai-models')
  expect(aiModelsRoute.options.beforeLoad).toBeUndefined()
  expect('path' in modelExperimentRoute.options && modelExperimentRoute.options.path).toBe(
    '/ai-models/experiments/$id',
  )
  expect(modelExperimentRoute.options.beforeLoad).toBeUndefined()
  expect(modelExperimentRoute.options.component).toBeTypeOf('function')
})
