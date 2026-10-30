import { afterEach, expect, it, vi } from 'vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { currentDestination, routeLocation } from './navigation'
import { readNavigationEntry } from '../model/navigation-entry'

function viewport(desktop: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: desktop && query.includes('64rem'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => true,
    onchange: null,
  }))
}
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  sessionStorage.clear()
  initializeI18n('ko')
})
it.each([
  ['/', undefined],
  ['/posts/new', '/naver-blog'],
  ['/clips/new', '/naver-clip'],
  ['/clips/new?channel=instagram_clip', '/insta-clip'],
  ['/posts/example', '/naver-blog'],
  ['/clips/example', '/naver-clip'],
  ['/voices/example/materials', '/settings'],
  ['/tests', '/settings'],
  ['/tests/history', '/settings'],
  ['/tests/test', '/settings'],
  ['/ai-models/compare', '/settings'],
  ['/billing', '/settings'],
] as const)('resolves %s to %s', (path, section) => expect(currentDestination(path)).toBe(section))
it('places every post and clip under its channel and exposes deterministic hierarchy', () => {
  expect(currentDestination('/posts/minted')).toBe('/naver-blog')
  expect(currentDestination('/clips/minted')).toBe('/naver-clip')
  expect(currentDestination('/clips/minted', 'instagram_clip')).toBe('/insta-clip')
  expect(routeLocation('/voices/voice/materials').ancestors.map((x) => x.href)).toEqual([
    '/settings',
    '/settings#settings-personal',
    '/voices',
    '/voices/voice',
  ])
  expect(routeLocation('/tests/id').ancestors.map((x) => x.href)).toEqual([
    '/settings',
    '/settings#settings-advanced',
    '/tests',
  ])
})
it('shows primary destinations and current section on desktop without a hamburger or modal', async () => {
  viewport(true)
  const calls: string[] = []
  renderAppAt('/naver-blog', { user: { id: 'alice', plan: ProtoPlan.FREE }, calls })
  await screen.findByRole('heading', { name: '네이버 블로그', level: 1 })
  const nav = screen.getByRole('navigation', { name: '주요' })
  expect(
    within(nav)
      .getAllByRole('link')
      .map((x) => x.getAttribute('href')),
  ).toEqual(['/naver-blog', '/naver-clip', '/insta-clip'])
  expect(within(nav).getByRole('link', { name: '블로그' })).toHaveAttribute('aria-current', 'page')
  expect(screen.queryByRole('button', { name: '이동 메뉴' })).toBeNull()
  expect(screen.queryByRole('dialog', { name: '메뉴' })).toBeNull()
  expect(screen.getByRole('navigation', { name: '현재 위치' })).toHaveTextContent('네이버 블로그')
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
it.each([false, true])(
  'keeps exactly three channel links directly reachable, desktop=%s',
  async (desktop) => {
    viewport(desktop)
    const user = userEvent.setup()
    const calls: string[] = []
    const view = renderAppAt('/posts/new', { user: { id: 'alice', plan: ProtoPlan.FREE }, calls })
    const nav = await screen.findByRole('navigation', { name: '주요' })
    expect(
      within(nav)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['블로그', '클립', '인스타'])
    expect(within(nav).getByRole('link', { name: '블로그' })).toHaveClass('underline')
    expect(screen.queryByRole('button', { name: '이동 메뉴' })).toBeNull()
    await user.click(within(nav).getByRole('link', { name: '인스타' }))
    await screen.findByRole('heading', { name: '인스타 클립', level: 1 })
    expect(view.router.state.location.pathname).toBe('/insta-clip')
    expect(within(nav).getByRole('link', { name: '인스타' })).toHaveAttribute(
      'aria-current',
      'page',
    )
    await user.click(screen.getByRole('button', { name: '내 계정' }))
    await user.click(
      within(screen.getByRole('dialog', { name: '내 계정' })).getByRole('link', {
        name: '내 설정',
      }),
    )
    await screen.findByRole('heading', { name: '내 설정', level: 1 })
    expect(nav.querySelector('[aria-current]')).toBeNull()
    expect(calls.filter((call) => /^(Start|Create|Generate|Cancel)/.test(call))).toEqual([])
  },
)
it('retains settings parent access and filters across child navigation and history', async () => {
  viewport(true)
  const { router } = renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '내 설정' })
  await act(() => router.navigate({ to: '/voices' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/voices'))
  const breadcrumb = screen.getByRole('navigation', { name: '현재 위치' })
  expect(within(breadcrumb).getByRole('link', { name: '내 설정' })).toHaveAttribute(
    'href',
    '/settings',
  )
  expect(within(breadcrumb).getByRole('link', { name: '말투·기억' })).toHaveAttribute(
    'href',
    '/settings#settings-personal',
  )
  await act(() => router.navigate({ to: '/settings' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  expect(
    screen.getByRole('navigation', { name: '주요' }).querySelector('[aria-current]'),
  ).toBeNull()
})
it('keeps administration protected and discoverable only to its owner role', async () => {
  viewport(true)
  renderAppAt('/settings', { user: { id: 'operator', plan: ProtoPlan.MASTER } })
  expect(await screen.findByRole('link', { name: '관리자' })).toHaveAttribute('href', '/admin')
})
it('retains a settings origin through reload and restores its document position on explicit return', async () => {
  viewport(true)
  const calls: string[] = []
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(window, 'scrollY', 'get').mockReturnValue(480)
  vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(2400)
  const view = renderAppAt('/settings', { user: { id: 'alice' }, calls })
  await screen.findByRole('heading', { name: '내 설정' })
  await act(() => view.router.navigate({ to: '/voices' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/voices'))
  expect(readNavigationEntry('alice', '/voices')).toMatchObject({
    path: '/settings',
    scrollY: 480,
  })
  view.unmount()
  const reopened = renderAppAt('/voices', { transport: view.transport })
  await userEvent.click(
    within(await screen.findByRole('navigation', { name: '현재 위치' })).getByRole('link', {
      name: '내 설정',
    }),
  )
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/settings'))
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 480))
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
it.each([
  ['ko', '네이버 블로그', '네이버 클립', '인스타 클립'],
  ['en', 'Naver Blog', 'Naver Clip', 'Instagram Clip'],
] as const)(
  'keeps exactly three home channel entries in %s without object writes',
  async (locale, post, naver, insta) => {
    viewport(true)
    initializeI18n(locale)
    const calls: string[] = []
    renderAppAt('/', { user: { id: 'alice' }, calls })
    const main = await screen.findByRole('main')
    expect(within(main).getAllByRole('link')).toHaveLength(3)
    expect(within(main).getByRole('link', { name: post })).toHaveAttribute('href', '/posts/new')
    expect(within(main).getByRole('link', { name: naver })).toHaveAttribute(
      'href',
      '/clips/new?channel=naver_clip',
    )
    expect(within(main).getByRole('link', { name: insta })).toHaveAttribute(
      'href',
      '/clips/new?channel=instagram_clip',
    )
    expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
  },
)

it('derives named child locations and unknown-route behavior from the same hierarchy', () => {
  for (const path of ['/naver-blog', '/naver-clip', '/insta-clip']) {
    expect(routeLocation(path).ancestors).toEqual([])
    expect(routeLocation(path).destination).toBe(path)
  }
  expect(currentDestination('/setup')).toBeUndefined()
  expect(routeLocation('/posts/minted').ancestors.map((x) => x.href)).toEqual(['/naver-blog'])
  expect(routeLocation('/clips/minted', 'instagram_clip').ancestors.map((x) => x.href)).toEqual([
    '/insta-clip',
  ])
  expect(routeLocation('/admin/models')).toMatchObject({
    current: 'adminModels',
    destination: '/settings',
  })
  expect(routeLocation('/billing/checkout').ancestors.at(-1)?.href).toBe('/billing')
  expect(routeLocation('/missing')).toMatchObject({
    current: 'notFound',
    ancestors: [],
    destination: undefined,
  })
})
