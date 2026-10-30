import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { clipsSearchSchema } from '@/pages/clips'
import { PostsPage, postsSearchSchema } from '@/pages/posts'
import { authenticatedRoute } from './tree'
/** Each channel opens straight onto its own history (THEME-38, POST-43, CLIP-41). */
const naverBlogRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/naver-blog',
  validateSearch: postsSearchSchema,
  component: PostsPage,
})
const naverClipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/naver-clip',
  validateSearch: clipsSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/clips'), 'NaverClipHistoryPage'),
})
const instaClipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/insta-clip',
  validateSearch: clipsSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/clips'), 'InstaClipHistoryPage'),
})
export const channelRoutes = [naverBlogRoute, naverClipRoute, instaClipRoute]
