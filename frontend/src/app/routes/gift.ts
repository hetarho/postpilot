import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { hasActivePublicSession } from './publicGuard'
import { rootRoute } from './tree'

// Public, like /about: a direct child of the root route and NOT under the authenticated
// layout, because a gift link is opened by visitors who may have no account yet (GIFT-8).
// The loader only tells the page which way in to offer; the server decides on redeem.
export const giftRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/gift/$token',
  loader: ({ context }) => hasActivePublicSession(context),
  component: lazyRouteComponent(() => import('@/pages/gift'), 'GiftPage'),
})

/** The group's routes, in the order the tree adds them. */
export const giftRoutes = [giftRoute]
