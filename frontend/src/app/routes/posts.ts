import { createRoute, redirect } from '@tanstack/react-router'
import { postReviewSearchSchema, testRecordSearchSchema } from '@/pages/model-experiment'
import { editorSearchSchema } from '@/pages/editor'
import { postsSearchSchema } from '@/pages/posts'
import { NewDraftPage, PostEditorPage } from '@/pages/editor'
import { authenticatedRoute } from './tree'

/** The post history moved onto the channel itself (POST-43); a bookmark or an old return link
 *  lands there with its narrowing. */
export const postsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts',
  validateSearch: postsSearchSchema,
  beforeLoad: ({ search }) => {
    throw redirect({ to: '/naver-blog', search, replace: true })
  },
})

// A static segment outranks '$slug', so this route — not the editor below — is what
// '/posts/new' matches.
export const newDraftRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts/new',
  validateSearch: editorSearchSchema,
  component: NewDraftPage,
})

export const postEditorRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts/$slug',
  validateSearch: editorSearchSchema,
  component: PostEditorPage,
})

export const postExperimentRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
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
      (search.from === 'posts' ? `/naver-blog${filters.size ? '?' + filters : ''}` : undefined)
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
