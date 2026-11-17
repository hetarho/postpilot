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
/** The addresses of the earlier tabs. 만들기 and 내 스타일 became home — which starts a post and
 *  draws the writing kit — and 내 콘텐츠 split into the 글 and 영상 tabs, so each replaces itself
 *  with the place that now does its job. */
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
