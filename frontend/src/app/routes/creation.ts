import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { setupSearchSchema } from '@/pages/setup'
import { authenticatedRoute } from './tree'
export const libraryRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/library',
  beforeLoad: () => {
    throw redirect({ to: '/my', replace: true })
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
/** Guide: the three tabs that are new places rather than renamed ones — 만들기 (one question),
 *  내 글 (every post and clip) and 내 스타일 (what the owner taught the AI). */
export const createPageRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/create',
  component: lazyRouteComponent(() => import('@/pages/create'), 'CreatePage'),
})
export const myWorkRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/my',
  component: lazyRouteComponent(() => import('@/pages/my-work'), 'MyWorkPage'),
})
export const styleRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/style',
  component: lazyRouteComponent(() => import('@/pages/style'), 'StylePage'),
})
export const creationRoutes = [
  libraryRoute,
  settingsRoute,
  setupRoute,
  createPageRoute,
  myWorkRoute,
  styleRoute,
]
