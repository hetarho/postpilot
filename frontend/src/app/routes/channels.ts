import { createRoute, lazyRouteComponent, redirect } from '@tanstack/react-router'
import { clipsSearchSchema } from '@/pages/clips'
import { PostsPage, postsSearchSchema } from '@/pages/posts'
import { authenticatedRoute } from './tree'

/** The 글 list and the 영상 list are the 글 and 영상 tabs (POST-43, CLIP-41). */
const postsListRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts',
  validateSearch: postsSearchSchema,
  component: PostsPage,
})
const clipsListRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/clips',
  validateSearch: clipsSearchSchema,
  component: lazyRouteComponent(() => import('@/pages/clips'), 'ClipHistoryPage'),
})

// The earlier addresses replace themselves with the lists, keeping the narrowing. A video has no
// channel any more, so one stored in a link is dropped.
const legacyBlogRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/naver-blog',
  validateSearch: postsSearchSchema,
  beforeLoad: ({ search }) => {
    throw redirect({ to: '/posts', search, replace: true })
  },
})
const legacyNaverVideoRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/naver-clip',
  validateSearch: clipsSearchSchema,
  beforeLoad: ({ search: { q, status } }) => {
    throw redirect({ to: '/clips', search: { q, status }, replace: true })
  },
})
const legacyInstagramVideoRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/insta-clip',
  validateSearch: clipsSearchSchema,
  beforeLoad: ({ search: { q, status } }) => {
    throw redirect({ to: '/clips', search: { q, status }, replace: true })
  },
})

export const channelRoutes = [
  postsListRoute,
  clipsListRoute,
  legacyBlogRoute,
  legacyNaverVideoRoute,
  legacyInstagramVideoRoute,
]
