import { describe, expect, it } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router'
import {
  createFakeAuthTransport,
  createTestQueryClient,
  type FakeAuthBackend,
  type FakeAuthOptions,
} from '@/test/session'
import { AuthService } from '@/shared/api'
import { routeTree } from './router'

/** Mounts the REAL route tree against a fake backend. The guard is the thing under
 *  test, so nothing about the routing is stubbed. */
function renderApp(at: string, auth: FakeAuthOptions = {}) {
  return renderWith(at, createFakeAuthTransport(auth))
}

function renderWith(at: string, transport: FakeAuthBackend['transport']) {
  const queryClient: QueryClient = createTestQueryClient()
  const router = createRouter({
    routeTree,
    context: { queryClient, transport },
    history: createMemoryHistory({ initialEntries: [at] }),
  })

  render(
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>,
  )

  return { router, transport, queryClient }
}

describe('session guard', () => {
  // Job 02 A2 / plan 01 AC9.
  it('sends an unauthenticated visit to /login, remembering where it was going', async () => {
    const { router } = renderApp('/')

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(router.state.location.search).toEqual({ redirect: '/' })
    expect(await screen.findByRole('button', { name: '로그인' })).toBeInTheDocument()
  })

  it('lets a signed-in visit through and shows the account in the header', async () => {
    const { router } = renderApp('/', { user: { id: 'alice' } })

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(await screen.findByText('alice')).toBeInTheDocument()
  })

  it('bounces an already-signed-in visitor away from /login', async () => {
    const { router } = renderApp('/login', { user: { id: 'alice' } })

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })

  it('asks the server once per unauthenticated load, not once per route', async () => {
    const calls: string[] = []
    const { router } = renderApp('/', { calls })

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    // The guard asks; the login route then reads that answer from the cache.
    expect(calls.filter((c) => c === 'GetMe')).toHaveLength(1)
  })
})

describe('login screen', () => {
  // Job 02 A2, second half: the originally requested route loads after login.
  it('returns to the route the guard blocked', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/')
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))

    await user.type(await screen.findByLabelText('아이디'), 'alice')
    await user.type(screen.getByLabelText('비밀번호'), 'pw')
    await user.click(screen.getByRole('button', { name: '로그인' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
    expect(await screen.findByText('alice')).toBeInTheDocument()
  })

  // Job 02 A6.
  it('says the same thing for every failure', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/login', { loginFails: true })

    await user.type(await screen.findByLabelText('아이디'), 'ghost')
    await user.type(screen.getByLabelText('비밀번호'), 'pw')
    await user.click(screen.getByRole('button', { name: '로그인' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('아이디 또는 비밀번호가 맞지 않아요')
    // Nothing may hint at whether the id exists.
    expect(alert.textContent).not.toMatch(/ghost|없|존재|not found/i)
    expect(router.state.location.pathname).toBe('/login')
  })

  it.each([
    ['protocol-relative', '//evil.example.com'],
    // The backslash form is the one a "starts with a single slash" check waves through;
    // the URL parser folds it into a slash and it leaves the origin.
    ['backslash', '/\\evil.example.com'],
    ['absolute', 'https://evil.example.com'],
  ])('ignores a %s redirect param', async (_name, target) => {
    const user = userEvent.setup()
    const { router } = renderApp('/login?redirect=' + encodeURIComponent(target))

    await user.type(await screen.findByLabelText('아이디'), 'alice')
    await user.type(screen.getByLabelText('비밀번호'), 'pw')
    await user.click(screen.getByRole('button', { name: '로그인' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/'))
  })

  // The login form is what a user reaches for when the app is misbehaving, so it has to
  // render even when the session probe cannot be answered at all.
  it('still renders when the API is down', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(AuthService.method.getMe, () => {
        throw new ConnectError('down', Code.Unavailable)
      })
    })
    const { router } = renderWith('/login', transport)

    expect(await screen.findByRole('button', { name: '로그인' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
  })
})

describe('outage', () => {
  // An outage is not a logout. Bouncing to /login would tell the user something false
  // and cost them the page they were on.
  it('does not report an unreachable API as a signed-out session', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(AuthService.method.getMe, () => {
        throw new ConnectError('down', Code.Unavailable)
      })
    })
    const { router } = renderWith('/', transport)

    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(router.state.location.pathname).not.toBe('/login')
  })
})

describe('logout', () => {
  // A failed Logout leaves the cookie valid, so "you are logged out" would be a lie the
  // next navigation exposes.
  it('keeps the user in the app and says so when logout fails', async () => {
    const user = userEvent.setup()
    const { router } = renderApp('/', { user: { id: 'alice' }, logoutFails: true })
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))

    await user.click(await screen.findByRole('button', { name: '로그아웃' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('로그아웃하지 못했어요')
    expect(router.state.location.pathname).toBe('/')
  })

  // Job 02 A5.
  it('ends the session and lands on /login', async () => {
    const user = userEvent.setup()
    const { router, transport, queryClient } = renderApp('/', { user: { id: 'alice' } })
    await waitFor(() => expect(router.state.location.pathname).toBe('/'))

    await user.click(await screen.findByRole('button', { name: '로그아웃' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    // Navigating back must not find a cached session waiting.
    const { getMeQueryKey } = await import('@/entities/session')
    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeUndefined()
    await router.navigate({ to: '/' })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })
})
