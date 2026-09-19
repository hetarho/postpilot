import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { aboutSearchSchema } from '@/pages/about'
import { rootRoute } from './tree'

// Public, and structurally so: a direct child of the root route, beside /login and NOT under the
// authenticated layout below. No beforeLoad, no loader, no session branch — mounting /about must
// not issue GetMe, because the page exists for visitors who have no account at all (plan 15).
export const aboutRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/about',
  validateSearch: aboutSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/about'), 'AboutPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const marketingRoutes = [aboutRoute]
