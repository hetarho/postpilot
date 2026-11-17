import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { setupSearchSchema } from '@/pages/setup'
import { authenticatedRoute } from './tree'
export const libraryRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/library',
  beforeLoad: () => {
    throw redirect({ to: '/posts', replace: true })
  },
})
export const settingsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/settings',
  component: lazyRouteComponent(() => import('@/pages/settings'), 'SettingsPage'),
})
export const setupRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/setup',
  validateSearch: setupSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/setup'), 'SetupPage'),
})
/** The earlier hubs replace themselves with the places that took their jobs: 만들기 and 내 스타일
 *  with 홈 (the kit and its 글쓰기), 내 콘텐츠 with the 글 tab. */
export const createPageRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/create',
  beforeLoad: () => {
    throw redirect({ to: '/', replace: true })
  },
})
export const myWorkRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/my',
  beforeLoad: () => {
    throw redirect({ to: '/posts', replace: true })
  },
})
export const styleRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/style',
  beforeLoad: () => {
    throw redirect({ to: '/', replace: true })
  },
})
export const creationRoutes = [
  libraryRoute,
  settingsRoute,
  setupRoute,
  createPageRoute,
  myWorkRoute,
  styleRoute,
]
