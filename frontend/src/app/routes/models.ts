import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { aiModelsSearchSchema } from '@/pages/ai-models'
import { modelReviewSearchSchema } from '@/pages/model-experiment'
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
  component: lazyRouteComponent(() => import('@/pages/ai-models'), 'ModelComparisonPage'),
})

export const modelHistoryRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/experiments',
  component: lazyRouteComponent(() => import('@/pages/ai-models'), 'ModelHistoryPage'),
})

export const modelLeaderboardRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/leaderboard',
  component: lazyRouteComponent(() => import('@/pages/ai-models'), 'ModelLeaderboardPage'),
})

export const modelExperimentRoute = createRoute({
  getParentRoute: () => modelGroupRoute,
  path: '/ai-models/experiments/$id',
  validateSearch: modelReviewSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/model-experiment'), 'ModelExperimentPage'),
})

export const modelRoutes = [
  aiModelsRoute,
  modelComparisonRoute,
  modelHistoryRoute,
  modelLeaderboardRoute,
  modelExperimentRoute,
]
