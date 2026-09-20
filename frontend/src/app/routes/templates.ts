import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { writingGroupRoute } from './tree'

export const templatesRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/templates',
  component: lazyRouteComponent(() => import('@/pages/templates'), 'TemplatesPage'),
})

// `new` is declared before `$templateId` so the static path cannot be swallowed by the param,
// exactly as `/posts/new` sits before `/posts/$slug` below.
export const newTemplateRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/templates/new',
  component: lazyRouteComponent(() => import('@/pages/template'), 'TemplatePage'),
})

export const templateRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/templates/$templateId',
  component: lazyRouteComponent(() => import('@/pages/template'), 'TemplatePage'),
})

export const guidelinesRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/guidelines',
  component: lazyRouteComponent(() => import('@/pages/guidelines'), 'GuidelinesPage'),
})

export const memoriesRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
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
