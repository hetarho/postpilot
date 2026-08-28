import { describe, expect, it, vi } from 'vitest'
import { act, renderHook, waitFor } from '@testing-library/react'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { AuthService, GetMeResponseSchema } from '@/shared/api'
import {
  createFakeAuthBackend,
  createFakeAuthTransport,
  createTestQueryClient,
  withProviders,
} from '@/test/session'
import { getMeQueryKey, loadSession } from './session-queries'
import { useLogin } from './useLogin'
import { useLogout } from './useLogout'

/** A backend whose GetMe always fails with something that is NOT a 401. */
function createBrokenBackend() {
  return createRouterTransport(({ rpc }) => {
    rpc(AuthService.method.getMe, () => {
      throw new ConnectError('down', Code.Unavailable)
    })
  })
}

describe('session cache', () => {
  // The whole point of the key construction: the hooks and the router guard must land on
  // ONE cache entry. If they don't, a successful login leaves the guard still seeing no
  // session and bouncing the user straight back to /login.
  it('login seeds the entry the router guard reads', async () => {
    const transport = createFakeAuthTransport()
    const queryClient = createTestQueryClient()
    const { result } = renderHook(() => useLogin(), {
      wrapper: withProviders(transport, queryClient),
    })

    await act(async () => {
      await result.current.mutateAsync({ loginId: 'alice', password: 'pw' })
    })

    expect(queryClient.getQueryData(getMeQueryKey(transport))).toMatchObject({
      user: { id: 'alice' },
    })
    await expect(loadSession(queryClient, transport)).resolves.toEqual({
      status: 'active',
      user: { id: 'alice' },
    })
  })

  it('logout drops the entry, so the guard stops letting the user through', async () => {
    const { transport, expireSession } = createFakeAuthBackend({ user: { id: 'alice' } })
    const queryClient = createTestQueryClient()

    await loadSession(queryClient, transport)
    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeDefined()

    const { result } = renderHook(() => useLogout(), {
      wrapper: withProviders(transport, queryClient),
    })
    await act(async () => {
      await result.current.mutateAsync({})
    })

    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeUndefined()
    expireSession()
    await expect(loadSession(queryClient, transport)).resolves.toEqual({ status: 'signed-out' })
  })

  // A failed Logout leaves the cookie valid. Dropping the local entry would be a lie the
  // next navigation exposes: the guard probes, finds the live session, and bounces the
  // user back in with no way to leave.
  it('keeps the entry when the logout call fails', async () => {
    const transport = createFakeAuthTransport({ user: { id: 'alice' }, logoutFails: true })
    const queryClient = createTestQueryClient()
    await loadSession(queryClient, transport)

    const { result } = renderHook(() => useLogout(), {
      wrapper: withProviders(transport, queryClient),
    })
    await act(async () => {
      await result.current.mutateAsync({}).catch(() => {})
    })

    await waitFor(() => expect(result.current.isError).toBe(true))
    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeDefined()
  })
})

describe('loadSession', () => {
  it('reports a live session', async () => {
    const transport = createFakeAuthTransport({ user: { id: 'alice' } })
    await expect(loadSession(createTestQueryClient(), transport)).resolves.toEqual({
      status: 'active',
      user: { id: 'alice' },
    })
  })

  it('reports signed-out for a 401 rather than throwing', async () => {
    const transport = createFakeAuthTransport()
    await expect(loadSession(createTestQueryClient(), transport)).resolves.toEqual({
      status: 'signed-out',
    })
  })

  // A 200 with no user is not a session. Treating it as one would open every protected
  // route to a malformed or half-migrated response.
  it('reports signed-out for a 200 that carries no user', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(AuthService.method.getMe, () => create(GetMeResponseSchema, {}))
    })
    await expect(loadSession(createTestQueryClient(), transport)).resolves.toEqual({
      status: 'signed-out',
    })
  })

  // An outage is not a logout. Reporting it as one would strand the user on a login form
  // that cannot possibly work, having lost the page they were on.
  it('throws for a failure that is not a 401', async () => {
    await expect(loadSession(createTestQueryClient(), createBrokenBackend())).rejects.toThrow()
  })

  it('keeps throwing an outage instead of reading it back as signed-out', async () => {
    const queryClient = createTestQueryClient()
    const transport = createBrokenBackend()

    await expect(loadSession(queryClient, transport)).rejects.toThrow()
    // The cached-answer shortcut must not treat this error as "no session".
    await expect(loadSession(queryClient, transport)).rejects.toThrow()
  })

  it('does not retry a 401', async () => {
    const calls: string[] = []
    const transport = createFakeAuthTransport({ calls })

    await loadSession(createTestQueryClient(), transport)

    expect(calls).toEqual(['GetMe'])
  })

  // The login route's reverse guard runs immediately after the route guard bounced the
  // user here. Re-asking would cost a second GetMe on every unauthenticated page load.
  it('answers a repeat question from the cached 401 instead of asking again', async () => {
    const calls: string[] = []
    const transport = createFakeAuthTransport({ calls })
    const queryClient = createTestQueryClient()

    await loadSession(queryClient, transport)
    await loadSession(queryClient, transport)

    expect(calls.filter((c) => c === 'GetMe')).toHaveLength(1)
  })

  // A session revoked elsewhere (another tab, an expiry, the operator) must stop
  // granting access without waiting for a full page reload.
  it('re-checks with the server once the cached session goes stale', async () => {
    vi.useFakeTimers()
    try {
      const { transport, expireSession } = createFakeAuthBackend({ user: { id: 'alice' } })
      const queryClient = createTestQueryClient()

      await expect(loadSession(queryClient, transport)).resolves.toMatchObject({
        status: 'active',
      })

      expireSession()
      // Inside the staleness window the cached answer still stands.
      await expect(loadSession(queryClient, transport)).resolves.toMatchObject({
        status: 'active',
      })

      await vi.advanceTimersByTimeAsync(31_000)
      await expect(loadSession(queryClient, transport)).resolves.toEqual({
        status: 'signed-out',
      })
    } finally {
      vi.useRealTimers()
    }
  })
})
