import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { aiModelsSearchSchema } from '@/pages/ai-models'
import { modelReviewSearchSchema } from '@/pages/model-experiment'
import {
  writingTestSearchSchema,
  writingTestHistorySearchSchema,
  type WritingTestSearch,
  type WritingTestHistorySearch,
} from '@/pages/writing-tests'
import { safeInternalPath } from '@/shared/lib/navigation'
import { authenticatedRoute } from './tree'
import { ContentGroupLayout } from './ContentGroupLayout'

export const modelGroupRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  id: 'models',
  validateSearch: aiModelsSearchSchema,
  component: ContentGroupLayout,
})

export const aiModelsRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models',
  component: lazyRouteComponent(() => import('@/pages/ai-models'), 'AIModelsPage'),
})

export const modelComparisonRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/compare',
  validateSearch: legacyModelComparisonSearchSchema,
  beforeLoad: ({ search }) => redirectLegacyModelComparison(search),
})

export const modelHistoryRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/experiments',
  validateSearch: writingTestHistorySearchSchema,
  beforeLoad: ({ search }) => redirectLegacyModelHistory(search),
})

export const modelLeaderboardRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/leaderboard',
  validateSearch: writingTestHistorySearchSchema,
  beforeLoad: ({ search }) => redirectLegacyModelHistory(search),
})

export const modelExperimentRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/experiments/$id',
  validateSearch: (search): { from?: 'compare'; entry?: string } => ({
    ...modelReviewSearchSchema(search),
    entry:
      typeof search.entry === 'string' && safeInternalPath(search.entry) ? search.entry : undefined,
  }),
  component: lazyRouteComponent(() => import('@/pages/model-experiment'), 'ModelExperimentPage'),
})

export function legacyModelComparisonSearchSchema(raw: Record<string, unknown>): WritingTestSearch {
  return writingTestSearchSchema({
    ...raw,
    factor: raw.stage === 'voice' ? 'voice' : 'model',
    stage: raw.stage === 'observe' ? 'observe' : 'write',
    count: 2,
  })
}

export function redirectLegacyModelComparison(search: WritingTestSearch): never {
  throw redirect({
    to: '/tests',
    search: writingTestSearchSchema({ ...search }),
    replace: true,
  })
}

export function redirectLegacyModelHistory(search: WritingTestHistorySearch): never {
  throw redirect({
    to: '/tests/history',
    search: writingTestHistorySearchSchema({ ...search }),
    replace: true,
  })
}

export const modelRoutes = [
  aiModelsRoute,
  modelComparisonRoute,
  modelHistoryRoute,
  modelLeaderboardRoute,
  modelExperimentRoute,
]
