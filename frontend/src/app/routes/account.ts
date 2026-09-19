import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { authenticatedRoute } from './tree'

export const accountRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/account',
  component: lazyRouteComponent(() => import('@/pages/account'), 'AccountPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const accountRoutes = [accountRoute]
