import { afterEach, describe, expect, it } from 'vitest'
import { cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { AuthService } from '@/shared/api'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

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
      '아이디 또는 비밀번호가 맞지 않아요.',
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
  // and cost them the page they were on. The root boundary also must not expose the
  // Connect message: it is private backend prose, even when TanStack catches it.
  it.each([
    ['ko', '요청을 마치지 못했어요. 다시 시도해 주세요.', '다시 시도'],
    ['en', 'Could not complete the request. Please try again.', 'Try again'],
  ] as const)('renders a safe %s route failure without raw prose', async (locale, copy, retry) => {
    initializeI18n(locale)
    const { router } = renderAppAt('/posts', { transport: createBrokenTransport() })

    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(router.state.location.pathname).not.toBe('/login')
    expect(await screen.findByRole('alert')).toHaveTextContent(copy)
    expect(screen.getByRole('button', { name: retry })).toBeInTheDocument()
    expect(document.body).not.toHaveTextContent('down')
    expect(document.body).not.toHaveTextContent('[unavailable]')
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

// Plan 11 A11/A12: 용도 is a top-level destination beside 말투, reachable from the nav.
describe('the purpose management route', () => {
  it('mounts /purposes and offers it in the navigation after 말투', async () => {
    renderAppAt('/purposes', { user: { id: 'alice' } })

    expect(await screen.findByRole('heading', { level: 1, name: '용도' })).toBeInTheDocument()
    // One list, so the phone tab bar and the desktop header cannot disagree; the entry sits
    // after 말투 in it.
    const labels = screen
      .getAllByRole('link')
      .map((link) => link.getAttribute('href'))
      .filter((href): href is string => href !== null)
    expect(labels.indexOf('/purposes')).toBeGreaterThan(labels.indexOf('/voices'))
  })
})

// Job 31: every screen outside the login → posts → editor core is now fetched when its route
// is first entered. These mount the app DIRECTLY at each address — createMemoryHistory starts
// there with no prior navigation, which is what a deep link or a refresh does — so a lazy
// boundary that only resolves via in-app navigation would fail here.
describe('lazily loaded routes', () => {
  it.each([
    ['/publishing-agents', '발행 Mac'],
    ['/voices', '말투'],
    ['/purposes', '용도'],
    ['/ai-models', 'AI 모델'],
  ])('renders %s on a direct load', async (path, heading) => {
    const { router } = renderAppAt(path, { user: { id: 'alice' } })

    expect(await screen.findByRole('heading', { level: 1, name: heading })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe(path)
  })

  // The five voice tabs share one chunk, so reaching any tab proves the whole tab area
  // resolved; the tab row proves VoiceLayout's separate boundary resolved with it.
  it('renders a voice tab and its layout on a direct load', async () => {
    const { router } = renderAppAt('/voices/voice-default/versions', { user: { id: 'alice' } })

    expect(await screen.findByRole('link', { name: '프로필' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '버전 기록' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/voices/voice-default/versions')
  })
})

// Job 32 A3/A17: this table mirrors every concrete path registered in routeTree. The pathless
// authenticated layout is exercised by every signed-in row; `/voices/$voiceId` is represented by
// its index and all five child paths; the two redirect-only registrations assert their landing
// paths. Running the same real route tree in both locales catches a catalog key that exists but is
// wired to the wrong page just as reliably as a missing translation.
describe('localized registered-route smoke', () => {
  const copy = {
    ko: {
      login: '로그인',
      posts: '내 글',
      publishing: '발행 Mac',
      voices: '말투',
      purposes: '용도',
      profile: '프로필',
      versions: '버전 기록',
      import: '기존 글 가져오기',
      rules: '대조 규칙',
      validations: '프로필 검증',
      models: 'AI 모델',
      retry: '다시 시도',
      title: '제목',
    },
    en: {
      login: 'Log in',
      posts: 'My posts',
      publishing: 'Publishing Macs',
      voices: 'Voices',
      purposes: 'Purposes',
      profile: 'Profile',
      versions: 'Version history',
      import: 'Import existing posts',
      rules: 'Contrast rules',
      validations: 'Profile validation',
      models: 'AI models',
      retry: 'Try again',
      title: 'Title',
    },
  } as const

  it.each([
    ['ko', 360],
    ['ko', 1280],
    ['en', 360],
    ['en', 1280],
  ] as const)(
    'mounts every registered route with %s copy at the %ipx structural breakpoint',
    async (locale, width) => {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
      window.dispatchEvent(new Event('resize'))
      initializeI18n(locale)
      const text = copy[locale]
      const cases: Array<{
        path: string
        role: 'button' | 'heading' | 'textbox'
        name: string
        signedIn?: boolean
        expectedPath?: string
      }> = [
        { path: '/login', role: 'button', name: text.login },
        { path: '/', role: 'heading', name: text.posts, signedIn: true, expectedPath: '/posts' },
        { path: '/posts', role: 'heading', name: text.posts, signedIn: true },
        {
          path: '/publishing-agents',
          role: 'heading',
          name: text.publishing,
          signedIn: true,
        },
        { path: '/voices', role: 'heading', name: text.voices, signedIn: true },
        { path: '/purposes', role: 'heading', name: text.purposes, signedIn: true },
        {
          path: '/voices/voice-default',
          role: 'heading',
          name: text.profile,
          signedIn: true,
        },
        {
          path: '/voices/voice-default/versions',
          role: 'heading',
          name: text.versions,
          signedIn: true,
        },
        {
          path: '/voices/voice-default/import',
          role: 'heading',
          name: text.import,
          signedIn: true,
        },
        {
          path: '/voices/voice-default/rules',
          role: 'heading',
          name: text.rules,
          signedIn: true,
        },
        {
          path: '/voices/voice-default/validations',
          role: 'heading',
          name: text.validations,
          signedIn: true,
        },
        {
          path: '/voice',
          role: 'heading',
          name: text.profile,
          signedIn: true,
          expectedPath: '/voices/voice-default',
        },
        {
          path: '/voice/rules',
          role: 'heading',
          name: text.rules,
          signedIn: true,
          expectedPath: '/voices/voice-default/rules',
        },
        { path: '/ai-models', role: 'heading', name: text.models, signedIn: true },
        {
          path: '/ai-models/experiments/smoke',
          role: 'button',
          name: text.retry,
          signedIn: true,
        },
        {
          path: '/voices/voice-default/rules/smoke/compare',
          role: 'button',
          name: text.retry,
          signedIn: true,
        },
        {
          path: '/voices/voice-default/validations/smoke',
          role: 'button',
          name: text.retry,
          signedIn: true,
        },
        { path: '/posts/new', role: 'textbox', name: text.title, signedIn: true },
        { path: '/posts/smoke-post', role: 'textbox', name: text.title, signedIn: true },
      ]

      for (const routeCase of cases) {
        cleanup()
        const { router } = renderAppAt(routeCase.path, {
          user: routeCase.signedIn ? { id: 'alice' } : undefined,
          posts:
            routeCase.path === '/posts/smoke-post'
              ? { posts: [{ slug: 'smoke-post', title: 'Fixture title' }] }
              : undefined,
        })

        expect(
          await screen.findByRole(routeCase.role, { name: routeCase.name }),
          routeCase.path,
        ).toBeInTheDocument()
        expect(router.state.location.pathname).toBe(routeCase.expectedPath ?? routeCase.path)
        expect(document.documentElement.lang).toBe(locale)

        // JSDOM cannot calculate geometry, so this is deliberately a structural contract over
        // every real route at both acceptance widths. Bounded horizontal strips must contain
        // their own overscroll; the app/page must not introduce a competing vertical scroller;
        // and native action/field primitives retain the 44px role token at either breakpoint.
        const main = screen.getByRole('main')
        expect(main.querySelectorAll('[class~="overflow-y-auto"]'), routeCase.path).toHaveLength(0)
        for (const scroller of main.querySelectorAll('[class*="overflow-x-auto"]')) {
          expect(scroller, routeCase.path).toHaveClass('overscroll-x-contain')
        }
        expect(main.querySelector('[class~="w-screen"]'), routeCase.path).not.toBeInTheDocument()
        expect(
          main.querySelector('[class~="min-w-screen"]'),
          routeCase.path,
        ).not.toBeInTheDocument()

        const directControls = main.querySelectorAll(
          'button, select, textarea, input:not([type="checkbox"]):not([type="radio"]):not([type="file"]):not([type="hidden"])',
        )
        for (const control of directControls) {
          expect(control.className, `${routeCase.path}: ${control.tagName}`).toMatch(
            /(?:^|\s)(?:min-h-11|size-11)(?:\s|$)/,
          )
        }
        for (const choice of main.querySelectorAll('input[type="checkbox"], input[type="radio"]')) {
          expect(choice.closest('label')?.className, routeCase.path).toMatch(
            /(?:^|\s)min-h-11(?:\s|$)/,
          )
        }
      }
    },
    60_000,
  )

  it.each([360, 1280])(
    'keeps the authenticated composition single-scroll and responsive at %ipx',
    async (width) => {
      Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
      window.dispatchEvent(new Event('resize'))
      renderAppAt('/posts', { user: { id: 'alice' } })

      const main = await screen.findByRole('main')
      const shell = main.closest('.pb-nav')
      expect(shell).toHaveClass('flex', 'flex-1', 'flex-col', 'sm:pb-0')
      expect(shell?.querySelectorAll('[class~="overflow-y-auto"]')).toHaveLength(0)
      expect(screen.getAllByRole('navigation', { name: '주요' })).toHaveLength(2)
      expect(document.querySelector('nav.sm\\:hidden')).toBeInTheDocument()
      expect(document.querySelector('nav.hidden.sm\\:flex')).toBeInTheDocument()
    },
  )
})
