// Shared harness for tests that need a fake AuthService.
//
// Everything is built per test: the app's own transport and QueryClient are module
// singletons, and connect-query keys the cache by transport identity, so reusing them
// would leak one test's session into the next.
import type { ReactNode } from 'react'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { create } from '@bufbuild/protobuf'
import { AuthService, GetMeResponseSchema, LoginResponseSchema, LogoutResponseSchema } from '@/shared/api'
import { type FakePostsOptions, registerPostService } from './posts'
import { type FakeProvidersOptions, registerProviderService } from './providers'

export interface FakeAuthOptions {
  /** The account GetMe reports. `undefined` makes GetMe answer 401, like a real server
   *  with no session. */
  user?: { id: string }
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
      if (!session) throw new ConnectError('unauthenticated', Code.Unauthenticated)
      return create(GetMeResponseSchema, { user: session })
    })
    rpc(AuthService.method.login, (req) => {
      calls?.push('Login')
      if (loginFails) throw new ConnectError('invalid credentials', Code.Unauthenticated)
      session = { id: req.loginId }
      return create(LoginResponseSchema, { user: session })
    })
    rpc(AuthService.method.logout, () => {
      calls?.push('Logout')
      if (logoutFails) throw new ConnectError('unavailable', Code.Unavailable)
      session = undefined
      return create(LogoutResponseSchema, {})
    })
    registerPostService(router, { calls, ...options.posts })
    registerProviderService(router, { calls, ...options.providers })
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
export function withProviders(
  transport: FakeAuthBackend['transport'],
  queryClient: QueryClient,
) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <TransportProvider transport={transport}>
        <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
      </TransportProvider>
    )
  }
}
