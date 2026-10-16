import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { guidelinesSearchSchema } from '@/pages/guidelines'
import { newTemplateSearchSchema } from '@/pages/template'
import { authenticatedRoute } from './tree'

export const templatesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/templates',
  component: lazyRouteComponent(() => import('@/pages/templates'), 'TemplatesPage'),
})

// `new` is declared before `$templateId` so the static path cannot be swallowed by the param,
// exactly as `/posts/new` sits before `/posts/$slug` below.
export const newTemplateRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/templates/new',
  validateSearch: newTemplateSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/template'), 'TemplatePage'),
})

export const templateRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/templates/$templateId',
  component: lazyRouteComponent(() => import('@/pages/template'), 'TemplatePage'),
})

export const guidelinesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/guidelines',
  validateSearch: guidelinesSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/guidelines'), 'GuidelinesPage'),
})

export const memoriesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/memories',
  component: lazyRouteComponent(() => import('@/pages/memories'), 'MemoriesPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const templateRoutes = [
  templatesRoute,
  newTemplateRoute,
  templateRoute,
  guidelinesRoute,
  memoriesRoute,
]
