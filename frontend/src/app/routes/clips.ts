import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { clipChannelPath, DEFAULT_CLIP_CHANNEL } from '@/entities/clip-project'
import { clipCreationSearchSchema } from '@/pages/clip'
import { clipsSearchSchema } from '@/pages/clips'
import { videoTemplateSearchSchema } from '@/pages/video-template'
import { spokenCreationSearchSchema } from '@/pages/spoken-voices'
import { authenticatedRoute } from './tree'

/** Each clip channel's history moved onto the channel itself (CLIP-41); a bookmark or an old
 *  return link lands there with its narrowing. */
export const clipsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/clips',
  validateSearch: clipsSearchSchema,
  beforeLoad: ({ search: { channel, ...search } }) => {
    throw redirect({ to: clipChannelPath(channel ?? DEFAULT_CLIP_CHANNEL), search, replace: true })
  },
})

export const newClipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/clips/new',
  validateSearch: clipCreationSearchSchema,
  staticData: { groupNav: 'hidden' },
  component: lazyRouteComponent(() => import('@/pages/clip'), 'ClipPage'),
})

export const clipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/clips/$clipId',
  staticData: { groupNav: 'hidden' },
  component: lazyRouteComponent(() => import('@/pages/clip'), 'ClipPage'),
})

export const videoTemplatesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/video-templates',
  component: lazyRouteComponent(() => import('@/pages/video-templates'), 'VideoTemplatesPage'),
})

export const newVideoTemplateRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/video-templates/new',
  validateSearch: videoTemplateSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/video-template'), 'VideoTemplatePage'),
})

export const videoTemplateRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/video-templates/$templateId',
  component: lazyRouteComponent(() => import('@/pages/video-template'), 'VideoTemplatePage'),
})

export const videoGuidelinesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/video-guidelines',
  component: lazyRouteComponent(() => import('@/pages/video-guidelines'), 'VideoGuidelinesPage'),
})

export const spokenVoicesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/spoken-voices',
  component: lazyRouteComponent(() => import('@/pages/spoken-voices'), 'SpokenVoicesPage'),
})
export const newSpokenVoiceRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/spoken-voices/new',
  validateSearch: spokenCreationSearchSchema,
  staticData: { groupNav: 'hidden' },
  component: lazyRouteComponent(() => import('@/pages/spoken-voices'), 'NewSpokenVoicePage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const clipRoutes = [
  clipsRoute,
  spokenVoicesRoute,
  newSpokenVoiceRoute,
  newClipRoute,
  clipRoute,
  videoTemplatesRoute,
  newVideoTemplateRoute,
  videoTemplateRoute,
  videoGuidelinesRoute,
]
