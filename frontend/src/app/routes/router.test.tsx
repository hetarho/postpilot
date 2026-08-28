import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { AuthService } from '@/shared/api'
import { renderAppAt } from '@/test/app'

/** A backend that cannot answer at all — an outage, not a logout. */
function createBrokenTransport() {
  return createRouterTransport(({ rpc }) => {
    rpc(AuthService.method.getMe, () => {
      throw new ConnectError('down', Code.Unavailable)
    })
  })
}

describe('session guard', () => {
  // Job 02 A2 / plan 01 AC9.
  it('sends an unauthenticated visit to /login, remembering where it was going', async () => {
    const { router } = renderAppAt('/posts')

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(router.state.location.search).toEqual({ redirect: '/posts' })
    expect(await screen.findByRole('button', { name: '로그인' })).toBeInTheDocument()
  })

  it('lets a signed-in visit through and shows the account in the header', async () => {
    const { router } = renderAppAt('/posts', { user: { id: 'alice' } })

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
    expect(await screen.findByText('alice')).toBeInTheDocument()
  })

  it('bounces an already-signed-in visitor away from /login', async () => {
    const { router } = renderAppAt('/login', { user: { id: 'alice' } })

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
  })

  it('asks the server once per unauthenticated load, not once per route', async () => {
    const calls: string[] = []
    const { router } = renderAppAt('/posts', { calls })

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    // The guard asks; the login route then reads that answer from the cache.
    expect(calls.filter((c) => c === 'GetMe')).toHaveLength(1)
  })
})

// Job 04 A7: the post list is the app's home now that the scaffold ping page is gone.
describe('the app home', () => {
  it('sends / to the post list', async () => {
    const { router } = renderAppAt('/', { user: { id: 'alice' } })

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
    expect(await screen.findByRole('heading', { name: '내 글' })).toBeInTheDocument()
  })
})

describe('login screen', () => {
  // Job 02 A2, second half: the originally requested route loads after login.
  it('returns to the route the guard blocked', async () => {
    const user = userEvent.setup()
    const posts = { posts: [{ slug: '20260820-jeju', title: '제주 3일' }] }
    const { router } = renderAppAt('/posts/20260820-jeju', { posts })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))

    await user.type(await screen.findByLabelText('아이디'), 'alice')
    await user.type(screen.getByLabelText('비밀번호'), 'pw')
    await user.click(screen.getByRole('button', { name: '로그인' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts/20260820-jeju'))
    expect(await screen.findByDisplayValue('제주 3일')).toBeInTheDocument()
  })

  // Job 02 A6.
  it('says the same thing for every failure', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/login', { loginFails: true })

    await user.type(await screen.findByLabelText('아이디'), 'ghost')
    await user.type(screen.getByLabelText('비밀번호'), 'pw')
    await user.click(screen.getByRole('button', { name: '로그인' }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('아이디 또는 비밀번호가 맞지 않아요')
    expect(screen.getByLabelText('아이디')).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByLabelText('비밀번호')).toHaveAccessibleDescription(
      '아이디 또는 비밀번호가 맞지 않아요',
    )
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
    const { router } = renderAppAt('/login?redirect=' + encodeURIComponent(target))

    await user.type(await screen.findByLabelText('아이디'), 'alice')
    await user.type(screen.getByLabelText('비밀번호'), 'pw')
    await user.click(screen.getByRole('button', { name: '로그인' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
  })

  // The login form is what a user reaches for when the app is misbehaving, so it has to
  // render even when the session probe cannot be answered at all.
  it('still renders when the API is down', async () => {
    const { router } = renderAppAt('/login', { transport: createBrokenTransport() })

    expect(await screen.findByRole('button', { name: '로그인' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/login')
  })
})

describe('outage', () => {
  // An outage is not a logout. Bouncing to /login would tell the user something false
  // and cost them the page they were on.
  it('does not report an unreachable API as a signed-out session', async () => {
    const { router } = renderAppAt('/posts', { transport: createBrokenTransport() })

    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(router.state.location.pathname).not.toBe('/login')
  })
})

describe('logout', () => {
  // A failed Logout leaves the cookie valid, so "you are logged out" would be a lie the
  // next navigation exposes.
  it('keeps the user in the app and says so when logout fails', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts', { user: { id: 'alice' }, logoutFails: true })
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))

    await user.click(await screen.findByRole('button', { name: '로그아웃' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('로그아웃하지 못했어요')
    expect(router.state.location.pathname).toBe('/posts')
  })

  // A draft still being retried must not follow the device to the next account: the
  // retry would carry the previous user's text under the new user's cookie, and the
  // server would file it there.
  it('drops an unfinished draft when the session ends', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { router } = renderAppAt('/posts/new', {
      user: { id: 'alice' },
      posts: { calls, failSaves: 99 },
    })

    await user.type(await screen.findByLabelText('제목'), '비밀')
    const saves = () => calls.filter((call) => call === 'SavePostDraft').length
    await waitFor(() => expect(saves()).toBeGreaterThan(0), { timeout: 4_000 })

    await user.click(screen.getByRole('button', { name: '로그아웃' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))

    const afterLogout = saves()
    // Two backoff windows (1 s + 2 s) with room to spare.
    await new Promise((resolve) => setTimeout(resolve, 3_500))
    expect(saves()).toBe(afterLogout)
  }, 15_000)

  // Job 02 A5.
  it('ends the session and lands on /login', async () => {
    const user = userEvent.setup()
    const { router, transport, queryClient } = renderAppAt('/posts', {
      user: { id: 'alice' },
    })
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))

    await user.click(await screen.findByRole('button', { name: '로그아웃' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    // Navigating back must not find a cached session waiting.
    const { getMeQueryKey } = await import('@/entities/session')
    expect(queryClient.getQueryData(getMeQueryKey(transport))).toBeUndefined()
    await router.navigate({ to: '/posts' })
    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
  })
})
