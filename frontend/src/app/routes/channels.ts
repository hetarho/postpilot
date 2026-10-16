import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { authenticatedRoute } from './tree'
const naverBlogRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/naver-blog',
  component: lazyRouteComponent(() => import('@/pages/channel'), 'NaverBlogChannelPage'),
})
const naverClipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/naver-clip',
  component: lazyRouteComponent(() => import('@/pages/channel'), 'NaverClipChannelPage'),
})
const instaClipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/insta-clip',
  component: lazyRouteComponent(() => import('@/pages/channel'), 'InstaClipChannelPage'),
})
export const channelRoutes = [naverBlogRoute, naverClipRoute, instaClipRoute]
