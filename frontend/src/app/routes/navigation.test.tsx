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
  ['/', '/'],
  ['/posts/new', '/'],
  ['/clips/new', '/'],
  ['/posts/example', '/library'],
  ['/clips/example', '/library'],
  ['/voices/example/materials', '/settings'],
  ['/tests', '/tests'],
  ['/tests/history', '/tests'],
  ['/tests/test', '/tests'],
  ['/ai-models/compare', '/tests'],
  ['/billing', '/settings'],
] as const)('resolves %s to %s', (path, section) => expect(currentDestination(path)).toBe(section))
it('keeps minted creation distinct from a direct history record and exposes deterministic hierarchy', () => {
  expect(currentDestination('/posts/minted', true)).toBe('/')
  expect(currentDestination('/clips/minted', true)).toBe('/')
  expect(routeLocation('/voices/voice/materials').ancestors.map((x) => x.href)).toEqual([
    '/settings',
    '/settings#settings-writing',
    '/voices',
    '/voices/voice',
  ])
  expect(routeLocation('/tests/id').ancestors.map((x) => x.href)).toEqual(['/tests'])
})
it('shows primary destinations and current section on desktop without a hamburger or modal', async () => {
  viewport(true)
  const calls: string[] = []
  renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE }, calls })
  await screen.findByRole('heading', { name: '글 작업 내역' })
  const nav = screen.getByRole('navigation', { name: '주요' })
  expect(
    within(nav)
      .getAllByRole('link')
      .map((x) => x.getAttribute('href')),
  ).toEqual(['/', '/tests', '/settings', '/library'])
  expect(within(nav).getByRole('link', { name: '작업 내역' })).toHaveAttribute(
    'aria-current',
    'page',
  )
  expect(screen.queryByRole('button', { name: '이동 메뉴' })).toBeNull()
  expect(screen.queryByRole('dialog', { name: '메뉴' })).toBeNull()
  expect(screen.getByRole('navigation', { name: '현재 위치' })).toHaveTextContent('글 작업 내역')
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
it('uses a nonmodal edge-attached phone menu with visible dismissal, Escape and focus return', async () => {
  viewport(false)
  const user = userEvent.setup()
  renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '글 작업 내역' })
  const trigger = screen.getByRole('button', { name: '이동 메뉴' })
  await user.click(trigger)
  const menu = screen.getByRole('dialog', { name: '이동 메뉴' })
  expect(menu).not.toHaveAttribute('aria-modal', 'true')
  expect(menu).toHaveClass('right-0', 'top-full')
  expect(
    within(menu)
      .getAllByRole('link')
      .map((x) => x.getAttribute('href')),
  ).toEqual(['/', '/tests', '/settings', '/library'])
  expect(within(menu).getByRole('button', { name: '이동 메뉴 닫기' })).toBeInTheDocument()
  expect(menu.contains(document.activeElement)).toBe(true)
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog', { name: '이동 메뉴' })).toBeNull()
  expect(trigger).toHaveFocus()
})
it('retains settings parent access and filters across child navigation and history', async () => {
  viewport(true)
  const { router } = renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '설정' })
  await act(() => router.navigate({ to: '/templates' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/templates'))
  const breadcrumb = screen.getByRole('navigation', { name: '현재 위치' })
  expect(within(breadcrumb).getByRole('link', { name: '설정' })).toHaveAttribute(
    'href',
    '/settings',
  )
  expect(within(breadcrumb).getByRole('link', { name: '글 설정' })).toHaveAttribute(
    'href',
    '/settings#settings-writing',
  )
  await act(() => router.navigate({ to: '/settings' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  expect(screen.getByRole('navigation', { name: '주요' })).toHaveTextContent('설정')
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
  await screen.findByRole('heading', { name: '설정' })
  await act(() => view.router.navigate({ to: '/templates' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/templates'))
  expect(readNavigationEntry('alice', '/templates')).toMatchObject({
    path: '/settings',
    scrollY: 480,
  })
  view.unmount()
  const reopened = renderAppAt('/templates', { transport: view.transport })
  await userEvent.click(
    within(await screen.findByRole('navigation', { name: '현재 위치' })).getByRole('link', {
      name: '설정',
    }),
  )
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/settings'))
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 480))
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
it.each([
  ['ko', '새 글 작성하기', '새 클립 만들기'],
  ['en', 'Write a new post', 'Create a new clip'],
] as const)(
  'keeps exactly two home content actions in %s without object writes',
  async (locale, post, clip) => {
    viewport(true)
    initializeI18n(locale)
    const calls: string[] = []
    renderAppAt('/', { user: { id: 'alice' }, calls })
    const main = await screen.findByRole('main')
    expect(within(main).getAllByRole('link')).toHaveLength(2)
    expect(within(main).getByRole('link', { name: new RegExp(post) })).toHaveAttribute(
      'href',
      '/posts/new',
    )
    expect(within(main).getByRole('link', { name: new RegExp(clip) })).toHaveAttribute(
      'href',
      '/clips/new',
    )
    expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
  },
)

it('derives named child locations and unknown-route behavior from the same hierarchy', () => {
  for (const path of ['/', '/tests', '/settings', '/library']) {
    expect(routeLocation(path).ancestors).toEqual([])
    expect(routeLocation(path).destination).toBe(path)
  }
  expect(currentDestination('/setup')).toBe('/')
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
