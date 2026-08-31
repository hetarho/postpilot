// Shared harness for tests that need a fake AuthService.
//
// Everything is built per test: the app's own transport and QueryClient are module
// singletons, and connect-query keys the cache by transport identity, so reusing them
// would leak one test's session into the next.
import type { ReactNode } from 'react'
import { Code, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import {
  AuthService,
  GetMeResponseSchema,
  ProtoPlan,
  LoginResponseSchema,
  LogoutResponseSchema,
} from '@/shared/api'
import { type FakePostsOptions, registerPostService } from './posts'
import { type FakeProvidersOptions, registerProviderService } from './providers'
import { type FakeJobsOptions, registerGenerationService } from './jobs'
import { type FakeVoiceOptions, registerVoiceService } from './voice'
import { type FakeExperimentsOptions, registerExperimentService } from './experiments'
import { type FakePublishingOptions, registerPublishingService } from './publishing'
import { type FakeGuidelinesOptions, registerGuidelineService } from './guidelines'
import { type FakePurposesOptions, registerPurposeService } from './purposes'
import { type FakePlansOptions, registerPlanServices } from './plans'
import { connectAppError } from './app-error'

export interface FakeAuthOptions {
  /** The account GetMe reports. `undefined` makes GetMe answer 401, like a real server
   *  with no session.
   *
   *  `plan` defaults to `master` for the same reason migration 0013 backfills existing
   *  accounts to it: every test written before the ladder existed assumed an account with
   *  full authority, and that is what those accounts became. A test about a gated surface
   *  sets a lower tier explicitly. */
  user?: { id: string; plan?: ProtoPlan }
  /** Makes Login answer 401, like wrong credentials. */
  loginFails?: boolean
  /** Makes Logout fail, like an API that went away mid-session. */
  logoutFails?: boolean
  /** Records every procedure the transport was asked for. */
  calls?: string[]
  /** The PostService the signed-in screens call. Present by default (with no posts) so a
   *  routing test that lands on /posts is not reading an "unimplemented" error instead of
   *  the failure it is actually looking for. */
  posts?: FakePostsOptions
  /** The ProviderService (model catalog). Present by default with an empty registry. */
  providers?: FakeProvidersOptions
  /** The durable job service used by pages that mount active-job polling. */
  jobs?: FakeJobsOptions
  /** The acting account's voice profile and sample mutations. */
  voice?: FakeVoiceOptions
  experiments?: FakeExperimentsOptions
  publishing?: FakePublishingOptions
  /** The acting account's 용도 briefs. Present by default with none, so every screen that
   *  mounts the selector reads an empty directory rather than an "unimplemented" error. */
  purposes?: FakePurposesOptions
  /** The acting account's 작문 지침. Present by default with none, so a screen that mounts the
   *  list reads an empty one rather than an "unimplemented" error. */
  guidelines?: FakeGuidelinesOptions
  /** The plan ladder: the caller's own tier and usage, and the operator's account list. */
  plans?: FakePlansOptions
}

/** A fake backend plus the controls a test needs over it. */
export interface FakeAuthBackend {
  transport: ReturnType<typeof createRouterTransport>
  /** Ends the session server-side, the way an expiry or a logout elsewhere would.
   *  Without it a test cannot model a session that dies while the user is working. */
  expireSession: () => void
}

export function createFakeAuthBackend(options: FakeAuthOptions = {}): FakeAuthBackend {
  const { user, loginFails, logoutFails, calls } = options
  let session = user

  const transport = createRouterTransport((router) => {
    const { rpc } = router
    rpc(AuthService.method.getMe, () => {
      calls?.push('GetMe')
      if (!session) throw connectAppError('AUTH_REQUIRED', Code.Unauthenticated)
      return create(GetMeResponseSchema, {
        user: { id: session.id },
        plan: session.plan ?? ProtoPlan.MASTER,
      })
    })
    rpc(AuthService.method.login, (req) => {
      calls?.push('Login')
      if (loginFails) throw connectAppError('INVALID_CREDENTIALS', Code.Unauthenticated)
      session = { id: req.loginId, plan: user?.plan }
      return create(LoginResponseSchema, {
        user: { id: session.id },
        plan: session.plan ?? ProtoPlan.MASTER,
      })
    })
    rpc(AuthService.method.logout, () => {
      calls?.push('Logout')
      if (logoutFails) throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
      session = undefined
      return create(LogoutResponseSchema, {})
    })
    registerPostService(router, { calls, ...options.posts })
    registerProviderService(router, { calls, ...options.providers })
    registerGenerationService(router, { calls, ...options.jobs })
    registerVoiceService(router, { calls, ...options.voice })
    registerExperimentService(router, { calls, ...options.experiments })
    registerPublishingService(router, { calls, ...options.publishing })
    registerPurposeService(router, { calls, ...options.purposes })
    registerGuidelineService(router, { calls, ...options.guidelines })
    registerPlanServices(router, { plan: user?.plan, calls, ...options.plans })
  })

  return {
    transport,
    expireSession: () => {
      session = undefined
    },
  }
}

/** The common case: a fake backend when the test never needs to expire the session. */
export function createFakeAuthTransport(options: FakeAuthOptions = {}) {
  return createFakeAuthBackend(options).transport
}

/** A QueryClient with retries off — a 401 is an answer, and a retry would double every
 *  assertion about how many requests were made. */
export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
}

/** Provider order mirrors App.tsx: TransportProvider outside QueryClientProvider. */
export function withProviders(transport: FakeAuthBackend['transport'], queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </TransportProvider>
    )
  }
}
