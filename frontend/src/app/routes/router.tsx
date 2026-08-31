import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  createRoute,
  createRouter,
  lazyRouteComponent,
  redirect,
} from '@tanstack/react-router'
import { loadSession } from '@/entities/session'
import { defaultVoice, loadVoices } from '@/entities/voice'
import { NewDraftPage, PostEditorPage } from '@/pages/editor'
import { LoginPage } from '@/pages/login'
import { PostsPage } from '@/pages/posts'
import { transport } from '@/shared/api'
import { SIGNED_IN_HOME, isInAppPath } from '@/shared/lib'
import { queryClient } from '../providers/query-client'
import { AuthenticatedLayout } from './AuthenticatedLayout'
import { RootLayout } from './RootLayout'
import { RouteError } from './RouteError'
import { RoutePending } from './RoutePending'

// Everything outside the login → posts → editor path is fetched when its route is first
// entered. A session's first paint is the post list or the editor, so those stay eager;
// the remaining nine screens were costing every first paint on a mobile-first product.
// Only `component` moves behind the boundary — every beforeLoad and loader below still
// runs first, so a signed-out visitor is redirected without fetching a screen's code.
//
// The five voice tabs all name the same specifier, so the bundler emits one 23 kB chunk for
// the whole tab area rather than five: the tabs are one screen and are always reached
// together. VoiceLayout is lazy too but stays its own 2 kB chunk — it belongs to app/routes,
// not to the pages/voice slice, and sharing their chunk would mean moving it across layers.
const lazyVoice = <
  K extends
    | 'VoicePage'
    | 'VoiceVersionsPage'
    | 'VoiceImportPage'
    | 'VoiceRulesPage'
    | 'VoiceValidationsPage',
>(
  name: K,
) => lazyRouteComponent(() => import('@/pages/voice'), name)

/** What every route's `beforeLoad` can reach. The transport travels with the query
 *  client because the session cache key is built from it — a guard using a different
 *  transport would read a different cache entry than the hooks write. */
export interface RouterContext {
  queryClient: QueryClient
  transport: Transport
}

// The curried form is required: plain createRootRoute pins the context type to {} and
// then every `context.queryClient` below compiles as `any`.
const rootRoute = createRootRouteWithContext<RouterContext>()({
  component: RootLayout,
  errorComponent: RouteError,
})

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

const publishingAgentsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/publishing-agents',
  component: lazyRouteComponent(() => import('@/pages/publishing-agents'), 'PublishingAgentsPage'),
})

const voicesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voices',
  component: lazyRouteComponent(() => import('@/pages/voices'), 'VoicesPage'),
})

const purposesRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/purposes',
  component: lazyRouteComponent(() => import('@/pages/purposes'), 'PurposesPage'),
})

// The layout of one voice: the five tabs keep their own addresses under `/voices/$voiceId` and
// share the tab row. The two detail screens further down stay OUTSIDE it — they are full-width
// review surfaces with their own back link, not a sixth tab.
const voiceLayoutRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voices/$voiceId',
  component: lazyRouteComponent(() => import('./VoiceLayout'), 'VoiceLayout'),
})

const voiceRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/',
  component: lazyVoice('VoicePage'),
})

const voiceVersionsRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/versions',
  component: lazyVoice('VoiceVersionsPage'),
})

const voiceImportRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/import',
  component: lazyVoice('VoiceImportPage'),
})

const voiceRulesRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/rules',
  component: lazyVoice('VoiceRulesPage'),
})

const voiceValidationsRoute = createRoute({
  getParentRoute: () => voiceLayoutRoute,
  path: '/validations',
  component: lazyVoice('VoiceValidationsPage'),
})

/** The tabs an old `/voice/<tab>` link may name, so the redirect keeps the user on the same
 *  screen of the default voice. Anything else lands on the profile tab. */
const LEGACY_VOICE_TABS = new Set(['versions', 'import', 'rules', 'validations'])

/** Sends an old `/voice` address to the same tab of the account's default voice — read from the
 *  directory, never created here — or to the directory itself when there is none to show. Always
 *  throws (a redirect), so a caller's `await` is the whole guard. */
async function redirectLegacyVoice(
  context: RouterContext & { user: { id: string } },
  tab: string,
): Promise<never> {
  const voices = await loadVoices(context.queryClient, context.transport, context.user.id).catch(
    () => [],
  )
  const target = defaultVoice(voices)
  if (!target) throw redirect({ to: '/voices', replace: true })
  const params = { voiceId: target.id }
  switch (LEGACY_VOICE_TABS.has(tab) ? tab : '') {
    case 'versions':
      throw redirect({ to: '/voices/$voiceId/versions', params, replace: true })
    case 'import':
      throw redirect({ to: '/voices/$voiceId/import', params, replace: true })
    case 'rules':
      throw redirect({ to: '/voices/$voiceId/rules', params, replace: true })
    case 'validations':
      throw redirect({ to: '/voices/$voiceId/validations', params, replace: true })
    default:
      throw redirect({ to: '/voices/$voiceId', params, replace: true })
  }
}

// The address the app had before voices were plural. Bookmarks and the empty-profile warning
// of an older draft still point here.
const legacyVoiceRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice',
  beforeLoad: async ({ context }) => {
    await redirectLegacyVoice(context, '')
  },
})

const legacyVoiceTabRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voice/$',
  beforeLoad: async ({ context, params }) => {
    await redirectLegacyVoice(context, params._splat ?? '')
  },
})

const aiModelsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/ai-models',
  component: lazyRouteComponent(() => import('@/pages/ai-models'), 'AIModelsPage'),
})

const modelExperimentRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/ai-models/experiments/$id',
  component: lazyRouteComponent(() => import('@/pages/model-experiment'), 'ModelExperimentPage'),
})

const voiceRuleComparisonRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voices/$voiceId/rules/$id/compare',
  component: lazyRouteComponent(
    () => import('@/pages/voice-rule-comparison'),
    'VoiceRuleComparisonPage',
  ),
})

const voiceValidationRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/voices/$voiceId/validations/$id',
  component: lazyRouteComponent(() => import('@/pages/voice-validation'), 'VoiceValidationPage'),
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
    publishingAgentsRoute,
    voicesRoute,
    purposesRoute,
    voiceLayoutRoute.addChildren([
      voiceRoute,
      voiceVersionsRoute,
      voiceImportRoute,
      voiceRulesRoute,
      voiceValidationsRoute,
    ]),
    legacyVoiceRoute,
    legacyVoiceTabRoute,
    aiModelsRoute,
    modelExperimentRoute,
    voiceRuleComparisonRoute,
    voiceValidationRoute,
    newDraftRoute,
    postEditorRoute,
  ]),
])

export const router = createRouter({
  routeTree,
  context: { queryClient, transport },
  defaultPendingComponent: RoutePending,
})

// Register the router instance for type safety across the app (Link, useNavigate, …).
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
