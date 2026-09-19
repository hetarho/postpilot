import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { clipsSearchSchema } from '@/pages/clips'
import { videoGroupRoute } from './tree'

export const clipsRoute = createRoute({
  getParentRoute: () => videoGroupRoute,
  path: '/clips',
  validateSearch: clipsSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/clips'), 'ClipsPage'),
})

export const newClipRoute = createRoute({
  getParentRoute: () => videoGroupRoute,
  path: '/clips/new',
  staticData: { groupNav: 'hidden' },
  component: lazyRouteComponent(() => import('@/pages/clip'), 'ClipPage'),
})

export const clipRoute = createRoute({
  getParentRoute: () => videoGroupRoute,
  path: '/clips/$clipId',
  staticData: { groupNav: 'hidden' },
  component: lazyRouteComponent(() => import('@/pages/clip'), 'ClipPage'),
})

export const videoTemplatesRoute = createRoute({
  getParentRoute: () => videoGroupRoute,
  path: '/video-templates',
  component: lazyRouteComponent(() => import('@/pages/video-templates'), 'VideoTemplatesPage'),
})

export const newVideoTemplateRoute = createRoute({
  getParentRoute: () => videoGroupRoute,
  path: '/video-templates/new',
  component: lazyRouteComponent(() => import('@/pages/video-template'), 'VideoTemplatePage'),
})

export const videoTemplateRoute = createRoute({
  getParentRoute: () => videoGroupRoute,
  path: '/video-templates/$templateId',
  component: lazyRouteComponent(() => import('@/pages/video-template'), 'VideoTemplatePage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const clipRoutes = [
  clipsRoute,
  newClipRoute,
  clipRoute,
  videoTemplatesRoute,
  newVideoTemplateRoute,
  videoTemplateRoute,
]
