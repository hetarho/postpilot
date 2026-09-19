import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { authenticatedRoute, masterOnly } from './tree'

// Master-only for the same reason the nav entry is: every PublishingService procedure is
// refused to another tier, so a direct visit would otherwise mount a screen whose every
// request comes back MASTER_ONLY.
export const publishingAgentsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/publishing-agents',
  beforeLoad: masterOnly,
  component: lazyRouteComponent(() => import('@/pages/publishing-agents'), 'PublishingAgentsPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const publishingRoutes = [publishingAgentsRoute]
