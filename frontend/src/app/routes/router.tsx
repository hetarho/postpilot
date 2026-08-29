import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  redirect,
} from '@tanstack/react-router'
import { loadSession } from '@/entities/session'
import { NewDraftPage, PostEditorPage } from '@/pages/editor'
import { LoginPage } from '@/pages/login'
import { PostsPage } from '@/pages/posts'
import { VoicePage } from '@/pages/voice'
import { AIModelsPage } from '@/pages/ai-models'
import { ModelExperimentPage } from '@/pages/model-experiment'
import { VoiceRuleComparisonPage } from '@/pages/voice-rule-comparison'
import { VoiceValidationPage } from '@/pages/voice-validation'
import { transport } from '@/shared/api'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import { queryClient } from '../providers/query-client'
import { AuthenticatedLayout } from './AuthenticatedLayout'
import { RootLayout } from './RootLayout'

/** What every route's `beforeLoad` can reach. The transport travels with the query
 *  client because the session cache key is built from it — a guard using a different
 *  transport would read a different cache entry than the hooks write. */
export interface RouterContext {
  queryClient: QueryClient
  transport: Transport
}

// The curried form is required: plain createRootRoute pins the context type to {} and
// then every `context.queryClient` below compiles as `any`.
const rootRoute = createRootRouteWithContext<RouterContext>()({ component: RootLayout })

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/login',
  validateSearch: (search: Record<string, unknown>): { redirect?: string } => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  beforeLoad: async ({ context, search }) => {
    // Resolve first, then throw. A `throw redirect()` inside a try block would be caught
    // by that block's own catch and silently swallowed.
    //
    // Unlike the guard below, this route swallows an outage: the login form is what a
    // user reaches for when the app is misbehaving, so it must render even when the API
    // cannot answer at all. Submitting will then fail with a real message.
    const signedIn = await loadSession(context.queryClient, context.transport)
      .then((session) => session.status === 'active')
      .catch(() => false)

    if (signedIn) {
      throw redirect({
        to: isInAppPath(search.redirect) ? search.redirect : SIGNED_IN_HOME,
        replace: true,
      })
    }
  },
  component: LoginPage,
})

// Pathless layout route: `id` with no `path`, so its children keep their own URLs and
// only inherit the guard. Every authenticated screen is added under this one, which is
// what makes "protected by default" structural rather than a thing to remember.
const authenticatedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authenticated',
  beforeLoad: async ({ context, location }) => {
    // loadSession throws only for failures that are NOT an answer, so an API outage
    // reaches the error boundary instead of being mistaken for a logout and costing the
    // user the page they were on.
    const session = await loadSession(context.queryClient, context.transport)
    if (session.status !== 'active') {
      // location.href is the in-app path + search + hash (no origin), so it round trips
      // through the login form as the post-login destination.
      throw redirect({ to: '/login', search: { redirect: location.href }, replace: true })
    }
    return { user: session.user }
  },
  component: AuthenticatedLayout,
})

// Kept as a redirect rather than dropped: '/' is what a bookmark, a bare domain and an
// older remembered `?redirect=` all resolve to.
const indexRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: SIGNED_IN_HOME, replace: true })
  },
})

const postsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts',
  component: PostsPage,
})

const voiceRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice',
  component: VoicePage,
})

const aiModelsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/ai-models',
  component: AIModelsPage,
})

const modelExperimentRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/ai-models/experiments/$id',
  component: ModelExperimentPage,
})

const voiceRuleComparisonRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice/rules/$id/compare',
  component: VoiceRuleComparisonPage,
})

const voiceValidationRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice/validations/$id',
  component: VoiceValidationPage,
})

// A static segment outranks '$slug', so this route — not the editor below — is what
// '/posts/new' matches.
const newDraftRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts/new',
  component: NewDraftPage,
})

const postEditorRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/posts/$slug',
  component: PostEditorPage,
})

/** Exported so a test can mount the real tree against a fake transport. */
export const routeTree = rootRoute.addChildren([
  loginRoute,
  authenticatedRoute.addChildren([
    indexRoute,
    postsRoute,
    voiceRoute,
    aiModelsRoute,
    modelExperimentRoute,
    voiceRuleComparisonRoute,
    voiceValidationRoute,
    newDraftRoute,
    postEditorRoute,
  ]),
])

export const router = createRouter({ routeTree, context: { queryClient, transport } })

// Register the router instance for type safety across the app (Link, useNavigate, …).
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
