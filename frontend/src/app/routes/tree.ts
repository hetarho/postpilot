import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext, createRoute, redirect } from '@tanstack/react-router'
import { loadSession } from '@/entities/session'
import { SIGNED_IN_HOME } from '@/shared/lib'
import { AuthenticatedLayout } from './AuthenticatedLayout'
import { RootLayout } from './RootLayout'
import { RouteError } from './RouteError'
import { VideoLayout } from './VideoLayout'
import { WritingLayout } from './WritingLayout'

/** What every route's `beforeLoad` can reach. The transport travels with the query
 *  client because the session cache key is built from it — a guard using a different
 *  transport would read a different cache entry than the hooks write. */
export interface RouterContext {
  queryClient: QueryClient
  transport: Transport
}

// The curried form is required: plain createRootRoute pins the context type to {} and
// then every `context.queryClient` in a group file compiles as `any`.
export const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  errorComponent: RouteError,
})

// Pathless layout route: `id` with no `path`, so its children keep their own URLs and
// only inherit the guard. Every authenticated screen is added under this one, which is
// what makes "protected by default" structural rather than a thing to remember.
export const authenticatedRoute = createRoute({
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

export const writingGroupRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  id: 'writing',
  component: WritingLayout,
})

export const videoGroupRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  id: 'video',
  component: VideoLayout,
})

// Kept as a redirect rather than dropped: '/' is what a bookmark, a bare domain and an
// older remembered `?redirect=` all resolve to.
export const indexRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: SIGNED_IN_HOME, replace: true })
  },
})

/** Every authenticated screen is refused to a non-master account the same way: redirected
 *  rather than refused, because the account HAS a session and bouncing it to /login would be a
 *  lie. The redirect is UX only — every admin and publishing procedure is refused server-side
 *  for a non-master caller, whatever route the client managed to render. */
export function masterOnly({ context }: { context: { user?: { plan?: string } } }): void {
  if (context.user?.plan !== 'master') throw redirect({ to: SIGNED_IN_HOME, replace: true })
}
