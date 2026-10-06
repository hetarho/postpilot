import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { authenticatedRoute } from './tree'
export const libraryRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/library',
  component: lazyRouteComponent(() => import('@/pages/library'), 'LibraryPage'),
})
export const settingsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/settings',
  component: lazyRouteComponent(() => import('@/pages/settings'), 'SettingsPage'),
})
export const creationRoutes = [libraryRoute, settingsRoute]
