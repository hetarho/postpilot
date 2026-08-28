import { afterEach, describe, expect, it, vi } from 'vitest'
import { Code, ConnectError, createClient, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import { AuthService, GetMeResponseSchema, LoginResponseSchema } from './gen/postpilot/v1/auth_pb'
import { HealthService } from './gen/postpilot/v1/health_pb'
import { onUnauthenticated } from './auth-events'
import { authClient, credentialedFetch, unauthenticatedInterceptor } from './transport'

afterEach(() => {
  vi.restoreAllMocks()
})

// Job 02 A3 / plan 01 AC4: the session cookie is HttpOnly and cross-origin, so it is
// sent only because the request opts in. A Transport exposes none of its options, so
// this flag is assertable only on the fetch wrapper itself.
describe('credentialedFetch', () => {
  it('opts every request into sending cookies', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(null, { status: 204 }))

    await credentialedFetch('/api/whatever', { method: 'POST' })

    expect(fetchSpy).toHaveBeenCalledOnce()
    expect(fetchSpy.mock.calls[0][1]).toMatchObject({
      method: 'POST',
      credentials: 'include',
    })
  })

  it('keeps the caller options it is given', async () => {
    const fetchSpy = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue(new Response(null, { status: 204 }))
    const signal = new AbortController().signal

    await credentialedFetch('/api/whatever', { method: 'POST', signal, body: 'x' })

    expect(fetchSpy.mock.calls[0][1]).toMatchObject({ method: 'POST', signal, body: 'x' })
  })
})

describe('the app transport', () => {
  it('actually sends its requests through the credentialed wrapper', async () => {
    // Deleting `fetch: credentialedFetch` from createConnectTransport would silently
    // stop sending pp_session; asserting on credentialedFetch alone would not notice.
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({}), {
        status: 200,
        headers: { 'content-type': 'application/json' },
      }),
    )

    await authClient.getMe({})

    expect(fetchSpy).toHaveBeenCalledOnce()
    expect(fetchSpy.mock.calls[0][1]).toMatchObject({ credentials: 'include' })
  })
})

describe('unauthenticatedInterceptor', () => {
  /** Mounts the real interceptor over a fake backend — no network involved. */
  function transportWith(handlers: Parameters<typeof createRouterTransport>[0]) {
    return createRouterTransport(handlers, {
      transport: { interceptors: [unauthenticatedInterceptor] },
    })
  }

  it('announces a 401 from an ordinary RPC', async () => {
    const transport = transportWith(({ rpc }) => {
      rpc(HealthService.method.ping, () => {
        throw new ConnectError('nope', Code.Unauthenticated)
      })
    })
    let fired = 0
    const off = onUnauthenticated(() => {
      fired += 1
    })

    await expect(createClient(HealthService, transport).ping({})).rejects.toThrow()
    off()

    expect(fired).toBe(1)
  })

  it('stays quiet for Login, whose 401 means the password was wrong', async () => {
    const transport = transportWith(({ rpc }) => {
      rpc(AuthService.method.login, () => {
        throw new ConnectError('invalid credentials', Code.Unauthenticated)
      })
    })
    let fired = 0
    const off = onUnauthenticated(() => {
      fired += 1
    })

    await expect(
      createClient(AuthService, transport).login({ loginId: 'a', password: 'b' }),
    ).rejects.toThrow()
    off()

    // Announcing this would bounce the user off the very page they are standing on.
    expect(fired).toBe(0)
  })

  it('announces a 401 from GetMe, which is a session that stopped being valid', async () => {
    const transport = transportWith(({ rpc }) => {
      rpc(AuthService.method.getMe, () => {
        throw new ConnectError('unauthenticated', Code.Unauthenticated)
      })
    })
    let fired = 0
    const off = onUnauthenticated(() => {
      fired += 1
    })

    await expect(createClient(AuthService, transport).getMe({})).rejects.toThrow()
    off()

    // A background refetch nobody is watching still has to move the app.
    expect(fired).toBe(1)
  })

  it('stays quiet for a failure that is not a 401', async () => {
    const transport = transportWith(({ rpc }) => {
      rpc(HealthService.method.ping, () => {
        throw new ConnectError('boom', Code.Internal)
      })
    })
    let fired = 0
    const off = onUnauthenticated(() => {
      fired += 1
    })

    await expect(createClient(HealthService, transport).ping({})).rejects.toThrow()
    off()

    expect(fired).toBe(0)
  })

  it('passes a successful call through untouched', async () => {
    const transport = transportWith(({ rpc }) => {
      rpc(AuthService.method.login, () => create(LoginResponseSchema, { user: { id: 'alice' } }))
      rpc(AuthService.method.getMe, () => create(GetMeResponseSchema, { user: { id: 'alice' } }))
    })

    const res = await createClient(AuthService, transport).getMe({})

    expect(res.user?.id).toBe('alice')
  })
})
