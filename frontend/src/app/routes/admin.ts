import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { authenticatedRoute, masterOnly } from './tree'

// Master-only, and redirected rather than refused: the account HAS a session, so bouncing it to
// /login would be a lie. The redirect is UX only — every admin procedure is refused server-side
// for a non-master caller, whatever route the client managed to render.
// The three operator surfaces share one frame and one guard; each keeps its own address so a tab
// is bookmarkable and the back button moves between them.
export const adminRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/admin',
  beforeLoad: masterOnly,
  component: lazyRouteComponent(() => import('./AdminLayout'), 'AdminLayout'),
})

export const adminAccountsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  component: lazyRouteComponent(() => import('@/pages/admin'), 'AdminPage'),
})

export const adminModelsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/models',
  component: lazyRouteComponent(() => import('@/pages/admin'), 'AdminModelsPage'),
})

export const adminEstimatorRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/estimator',
  component: lazyRouteComponent(() => import('@/pages/admin'), 'AdminEstimatorPage'),
})

export const adminVouchersRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/vouchers',
  component: lazyRouteComponent(() => import('@/pages/admin'), 'AdminVouchersPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const adminRoutes = [
  adminAccountsRoute,
  adminModelsRoute,
  adminEstimatorRoute,
  adminVouchersRoute,
]
