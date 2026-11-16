import { createRoute, redirect } from '@tanstack/react-router'
import { authenticatedRoute } from './tree'

/** Compatibility for bookmarks from the retired automatic-publishing surface. The parent route
 * authenticates first, then this route redirects without loading a page or issuing an RPC. */
export const legacyPublishingAgentsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/publishing-agents',
  beforeLoad: () => {
    throw redirect({ to: '/posts', replace: true })
  },
})

export const legacyPublishingRoutes = [legacyPublishingAgentsRoute]
