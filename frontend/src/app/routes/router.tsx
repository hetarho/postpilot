import { createRouter } from '@tanstack/react-router'
import { transport } from '@/shared/api'
import { queryClient } from '../providers/query-client'
import { RoutePending } from './RoutePending'
import { scrollRestorationKey } from './scroll-restoration'
import { accountRoutes } from './account'
import { adminRoute, adminRoutes } from './admin'
import { authRoutes } from './auth'
import { billingRoutes } from './billing'
import { clipRoutes } from './clips'
import { giftRoutes } from './gift'
import { marketingRoutes } from './marketing'
import { modelGroupRoute, modelRoutes } from './models'
import { postRoutes } from './posts'
import { legacyPublishingRoutes } from './publishing'
import { templateRoutes } from './templates'
import {
  authenticatedRoute,
  indexRoute,
  rootRoute,
  videoGroupRoute,
  writingGroupRoute,
} from './tree'
import { legacyVoiceRoutes, voiceLayoutRoute, voiceRoutes, voiceTabRoutes } from './voices'

export type { RouterContext } from './tree'

/** The whole address space of the product, assembled from one file per group. A new screen
 *  edits its group and its page; nothing else knows it exists (ARCH-16).
 *
 *  Exported so a test can mount the real tree against a fake transport. */
export const routeTree = rootRoute.addChildren([
  ...authRoutes,
  ...marketingRoutes,
  ...giftRoutes,
  authenticatedRoute.addChildren([
    indexRoute,
    writingGroupRoute.addChildren([
      ...postRoutes,
      ...voiceRoutes,
      ...templateRoutes,
      voiceLayoutRoute.addChildren(voiceTabRoutes),
    ]),
    videoGroupRoute.addChildren(clipRoutes),
    ...legacyPublishingRoutes,
    adminRoute.addChildren(adminRoutes),
    ...billingRoutes,
    ...accountRoutes,
    ...legacyVoiceRoutes,
    modelGroupRoute.addChildren(modelRoutes),
  ]),
])

export const router = createRouter({
  routeTree,
  context: { queryClient, transport },
  defaultPendingComponent: RoutePending,
  scrollRestoration: true,
  getScrollRestorationKey: scrollRestorationKey,
})

// Register the router instance for type safety across the app (Link, useNavigate, …).
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
  /** What a route tells the chrome around it. `groupNav: 'hidden'` takes the group's second level
   *  off a page that is a workspace rather than a destination (→CLIP-37). */
  interface StaticDataRouteOption {
    groupNav?: 'hidden'
  }
  interface HistoryState {
    notice?: 'password-changed'
    billingRegistration?: { cardLabel: string; bonusGranted: boolean }
    billingSubscription?: { tier: string; changed?: boolean }
  }
}
