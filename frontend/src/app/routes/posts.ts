import { createRoute, lazyRouteComponent } from '@tanstack/react-router'
import { postReviewSearchSchema } from '@/pages/model-experiment'
import { postsSearchSchema } from '@/pages/posts'
import { NewDraftPage, PostEditorPage } from '@/pages/editor'
import { PostsPage } from '@/pages/posts'
import { writingGroupRoute } from './tree'

export const postsRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/posts',
  validateSearch: postsSearchSchema,
  component: PostsPage,
})

// A static segment outranks '$slug', so this route — not the editor below — is what
// '/posts/new' matches.
export const newDraftRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/posts/new',
  component: NewDraftPage,
})

export const postEditorRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/posts/$slug',
  component: PostEditorPage,
})

export const postExperimentRoute = createRoute({
  getParentRoute: () => writingGroupRoute,
  path: '/posts/experiments/$id',
  validateSearch: (search) => ({
    ...postReviewSearchSchema(search),
    ...postsSearchSchema(search),
  }),
  component: lazyRouteComponent(() => import('@/pages/model-experiment'), 'PostExperimentPage'),
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const postRoutes = [postsRoute, newDraftRoute, postExperimentRoute, postEditorRoute]
