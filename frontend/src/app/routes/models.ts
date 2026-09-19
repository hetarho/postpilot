import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { authenticatedRoute } from './tree'

export const aiModelsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/ai-models',
  component: lazyRouteComponent(() => import('@/pages/ai-models'), 'AIModelsPage'),
})

export const modelExperimentRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/ai-models/experiments/$id',
  component: lazyRouteComponent(() => import('@/pages/model-experiment'), 'ModelExperimentPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const modelRoutes = [aiModelsRoute, modelExperimentRoute]
