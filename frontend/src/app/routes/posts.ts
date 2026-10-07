import { createRoute, redirect } from '@tanstack/react-router'
import { postReviewSearchSchema, testRecordSearchSchema } from '@/pages/model-experiment'
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
    ...testRecordSearchSchema(search),
  }),
  beforeLoad: ({ params, search }) => {
    const filters = new URLSearchParams()
    if (search.q) filters.set('q', search.q)
    if (search.status) filters.set('status', search.status)
    const entry =
      search.entry ??
      (search.from === 'posts' ? `/posts${filters.size ? '?' + filters : ''}` : undefined)
    throw redirect({
      to: '/tests/records/$id',
      params: { id: params.id },
      search: { entry, source: search.source },
      replace: true,
    })
  },
})

/** The group's routes, in the order the tree adds them: a static path always before the
 *  param that would otherwise swallow it. */
export const postRoutes = [postsRoute, newDraftRoute, postExperimentRoute, postEditorRoute]
