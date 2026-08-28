import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from '@tanstack/react-router'
import { waitFor } from '@testing-library/react'
import { emitUnauthenticated } from '@/shared/api'
import { getMeQueryKey } from '@/entities/session'
import { createFakeAuthBackend, createTestQueryClient } from '@/test/session'
import { routeTree } from '../routes/router'
import { registerAuthRedirect } from './auth-redirect'

function setup(at: string, user?: { id: string }) {
  const { transport, expireSession } = createFakeAuthBackend({ user })
  const queryClient = createTestQueryClient()
  const router = createRouter({
    routeTree,
    context: { queryClient, transport },
    history: createMemoryHistory({ initialEntries: [at] }),
  })
  return { router, transport, queryClient, expireSession }
}

describe('registerAuthRedirect', () => {
  // The route guard only runs on navigation, so a session that dies while the user sits
  // on a screen has to be caught here.
  it('sends the user to /login when a session dies mid-use', async () => {
    const { router, transport, queryClient, expireSession } = setup('/posts', { id: 'alice' })
    await router.load()
    expect(router.state.location.pathname).toBe('/posts')
    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeDefined()

    // The event only means anything if the session really is gone server-side; with a
    // live one, /login would (correctly) bounce the user straight back.
    expireSession()
    const off = registerAuthRedirect({ router, queryClient, transport })
    emitUnauthenticated()

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(router.state.location.search).toEqual({ redirect: '/posts' })
    // The stale session must go, or the guard would wave the user straight back through.
    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeUndefined()
    off()
  })

  // Without this, /login's own session probe would navigate to /login, whose beforeLoad
  // probes again — a loop.
  it('does not navigate when already on /login', async () => {
    const { router, transport, queryClient } = setup('/login')
    await router.load()

    const off = registerAuthRedirect({ router, queryClient, transport })
    let navigations = 0
    const unsubscribe = router.subscribe('onBeforeNavigate', () => {
      navigations += 1
    })

    emitUnauthenticated()
    emitUnauthenticated()
    await new Promise((resolve) => setTimeout(resolve, 20))

    expect(navigations).toBe(0)
    expect(router.state.location.pathname).toBe('/login')
    unsubscribe()
    off()
  })

  it('stops listening once unregistered', async () => {
    const { router, transport, queryClient, expireSession } = setup('/posts', { id: 'alice' })
    await router.load()

    const off = registerAuthRedirect({ router, queryClient, transport })
    off()
    expireSession()
    emitUnauthenticated()
    await new Promise((resolve) => setTimeout(resolve, 20))

    expect(router.state.location.pathname).toBe('/posts')
  })
})
