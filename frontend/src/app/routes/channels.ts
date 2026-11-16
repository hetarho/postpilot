import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
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
// Videos are one list now; an old Instagram address lands on it.
const instaClipRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/insta-clip',
  validateSearch: clipsSearchSchema,
  beforeLoad: ({ search }) => {
    throw redirect({ to: '/naver-clip', search, replace: true })
  },
})
export const channelRoutes = [naverBlogRoute, naverClipRoute, instaClipRoute]
