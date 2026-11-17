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
/** The earlier tab addresses: 만들기 and 내 스타일 became 홈, whose kit opens every style screen
 *  and whose 글쓰기 starts a post; 내 콘텐츠 split into the 글 and 영상 tabs. A bookmark or a
 *  stored return under one still lands somewhere that holds what it used to. */
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
