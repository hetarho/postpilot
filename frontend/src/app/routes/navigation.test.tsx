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
// Guide: four places. 홈 holds the writing kit (말투 · 템플릿 · 지침 · 기억 · 목소리 are its screens),
// 글 every post, 영상 every video, AI 토너먼트 the models. Account and settings sit behind the avatar
// and belong to no tab.
it.each([
  ['/', '/'],
  ['/posts/new', '/posts'],
  ['/posts', '/posts'],
  ['/posts/example', '/posts'],
  ['/clips/new', '/clips'],
  ['/clips', '/clips'],
  ['/clips/example', '/clips'],
  ['/voices', '/'],
  ['/voices/example/materials', '/'],
  ['/templates', '/'],
  ['/templates/new', '/'],
  ['/video-templates', '/'],
  ['/guidelines', '/'],
  ['/memories', '/'],
  ['/spoken-voices/new', '/'],
  ['/ai-models', '/ai-models'],
  ['/tests', '/ai-models'],
  ['/tests/history', '/ai-models'],
  ['/tests/test', '/ai-models'],
  ['/ai-models/compare', '/ai-models'],
  ['/settings', undefined],
  ['/billing', undefined],
  ['/plans', undefined],
  ['/admin/models', undefined],
] as const)('resolves %s to %s', (path, tab) => expect(currentDestination(path)).toBe(tab))
it('places every post and clip under its list and every kit screen under 홈', () => {
  expect(routeLocation('/posts/minted').ancestors.map((x) => x.href)).toEqual(['/posts'])
  expect(routeLocation('/clips/minted').ancestors.map((x) => x.href)).toEqual(['/clips'])
  expect(routeLocation('/voices/voice/materials').ancestors.map((x) => x.href)).toEqual([
    '/',
    '/voices',
    '/voices/voice',
  ])
  expect(routeLocation('/voices/voice').ancestors.map((x) => x.href)).toEqual(['/', '/voices'])
  expect(routeLocation('/memories').ancestors.map((x) => x.href)).toEqual(['/'])
  expect(routeLocation('/tests/id').ancestors.map((x) => x.href)).toEqual(['/ai-models', '/tests'])
  expect(routeLocation('/tests/records/id').ancestors.map((x) => x.href)).toEqual([
    '/ai-models',
    '/tests/history',
  ])
})
// Guide: the destinations are a bottom tab bar at every width, a floating pill on the desk. The
// writing screen is a focused flow: it hides the bar and keeps its own way back.
it.each([false, true])('keeps the four places one tap away, desktop=%s', async (desktop) => {
  viewport(desktop)
  const user = userEvent.setup()
  const calls: string[] = []
  const view = renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE }, calls })
  const nav = await screen.findByRole('navigation', { name: '주요' })
  const links = within(nav).getAllByRole('link')
  expect(links.map((link) => link.textContent)).toEqual(['홈', '글', '영상', 'AI 토너먼트'])
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
    '/',
    '/posts',
    '/clips',
    '/ai-models',
  ])
  // The 글 list is its own tab: current, told by the accent ink and aria-current, never by
  // colour alone, its highlight holding the icon and the name together.
  const posts = within(nav).getByRole('link', { name: '글' })
  expect(posts).toHaveAttribute('aria-current', 'page')
  expect(posts).toHaveClass('text-tab-bar-fg-current')
  const indicator = posts.querySelector('.bg-tab-bar-indicator')
  expect(indicator).not.toBeNull()
  expect(indicator).toContainElement(posts.querySelector('svg'))
  for (const name of ['홈', '영상', 'AI 토너먼트']) {
    const link = within(nav).getByRole('link', { name })
    expect(link).not.toHaveAttribute('aria-current')
    expect(link).toHaveClass('text-tab-bar-fg')
  }
  // The top bar is one slim row: the brand — the way home — and the avatar. The theme and the
  // language are behind the avatar at every width, not in the bar.
  const banner = screen.getByRole('banner')
  expect(banner.children).toHaveLength(1)
  expect(banner).toContainElement(screen.getByRole('button', { name: '내 계정' }))
  expect(banner.querySelector('a[href="/"]')).not.toBeNull()
  expect(within(banner).queryByRole('group', { name: '인터페이스 환경설정' })).toBeNull()
  expect(banner).not.toContainElement(nav)

  await user.click(within(nav).getByRole('link', { name: '홈' }))
  await screen.findByRole('heading', { name: /고품질의 글을/, level: 1 })
  expect(view.router.state.location.pathname).toBe('/')
  expect(within(nav).getByRole('link', { name: '홈' })).toHaveAttribute('aria-current', 'page')
  // The writing screen hides the bar; the avatar still reaches the settings.
  await act(() => view.router.navigate({ to: '/posts/new' }))
  await screen.findByRole('heading', { name: '어떤 글을 쓰시나요?', level: 2 })
  expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
  await user.click(screen.getByRole('button', { name: '내 계정' }))
  const panel = within(screen.getByRole('dialog', { name: '내 계정' }))
  expect(panel.getByRole('group', { name: '인터페이스 환경설정' })).toBeInTheDocument()
  await user.click(panel.getByRole('link', { name: '내 설정' }))
  await screen.findByRole('heading', { name: '내 설정', level: 1 })
  expect(
    screen.getByRole('navigation', { name: '주요' }).querySelector('[aria-current]'),
  ).toBeNull()
  expect(calls.filter((call) => /^(Start|Create|Generate|Cancel)/.test(call))).toEqual([])
})
// Guide: a kit screen sits under 홈 with one way back to it; 템플릿 and 지침 add a 글 · 영상 switch
// in that same row.
it('opens a kit screen under 홈 with one way back and the 글 · 영상 switch', async () => {
  viewport(false)
  renderAppAt('/templates', { user: { id: 'alice' } })
  const back = await screen.findByRole('navigation', { name: '돌아가기' })
  expect(within(back).getByRole('link', { name: '홈' })).toHaveAttribute('href', '/')
  const kinds = within(screen.getByRole('navigation', { name: '글 · 영상' }))
  expect(kinds.getByRole('link', { name: '글' })).toHaveAttribute('aria-current', 'page')
  expect(kinds.getByRole('link', { name: '영상' })).toHaveAttribute('href', '/video-templates')
  expect(screen.queryByRole('navigation', { name: '나누어 보기' })).toBeNull()
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '홈' }),
  ).toHaveAttribute('aria-current', 'page')
})
it('gives a settings child one way back to 내 설정, and remembers where it was opened from', async () => {
  viewport(true)
  const { router } = renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '내 설정', level: 1 })
  await act(() => router.navigate({ to: '/account' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/account'))
  // The page's frame draws the way back in its own column, on the title's row, and the shell
  // draws none of its own: one way back, not two.
  const page = await screen.findByRole('main')
  const back = within(page).getByRole('link', { name: '내 설정' })
  expect(back).toHaveAttribute('href', '/settings')
  expect(back.parentElement).toContainElement(
    within(page).getByRole('heading', { level: 1, name: '계정 설정' }),
  )
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()
  expect(screen.queryByRole('navigation', { name: '현재 위치' })).toBeNull()
})
it('keeps administration protected and discoverable only to its owner role', async () => {
  viewport(true)
  renderAppAt('/settings', { user: { id: 'operator', plan: ProtoPlan.MASTER } })
  expect(await screen.findByRole('link', { name: '관리자' })).toHaveAttribute('href', '/admin')
  cleanup()
  renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '내 설정', level: 1 })
  expect(screen.queryByRole('link', { name: '관리자' })).toBeNull()
})
it('retains a 내 설정 origin through reload and restores its document position on explicit return', async () => {
  viewport(true)
  const calls: string[] = []
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(window, 'scrollY', 'get').mockReturnValue(480)
  vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(2400)
  const view = renderAppAt('/settings', { user: { id: 'alice' }, calls })
  await screen.findByRole('heading', { name: '내 설정', level: 1 })
  await act(() => view.router.navigate({ to: '/account' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/account'))
  expect(readNavigationEntry('alice', '/account')).toMatchObject({
    path: '/settings',
    scrollY: 480,
  })
  view.unmount()
  const reopened = renderAppAt('/account', { transport: view.transport })
  await screen.findByRole('heading', { level: 1, name: '계정 설정' })
  await userEvent.click(within(screen.getByRole('main')).getByRole('link', { name: '내 설정' }))
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/settings'))
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 480))
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
// 홈 is the kit and one 글쓰기: every kit part is a way to its own screen, and nothing is written.
it.each([
  ['ko', '글쓰기 준비 상태', '글쓰기'],
  ['en', 'Writing kit readiness', 'Write'],
] as const)(
  'shows the writing kit and one 글쓰기 on 홈 in %s without object writes',
  async (locale, figure, write) => {
    viewport(true)
    initializeI18n(locale)
    const calls: string[] = []
    renderAppAt('/', { user: { id: 'alice' }, calls })
    const kit = within(await screen.findByRole('figure', { name: figure }))
    expect(kit.getAllByRole('link').map((link) => link.getAttribute('href'))).toEqual([
      '/templates',
      '/voices',
      '/guidelines',
      '/memories',
      // 쓰는 AI opens AI 토너먼트 on 지금 쓰는 AI, under the comparisons.
      '/ai-models#ai-in-use',
      '/spoken-voices',
    ])
    expect(screen.getByRole('link', { name: write })).toHaveAttribute('href', '/posts/new')
    expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
  },
)

it('derives named child locations and unknown-route behavior from the same hierarchy', () => {
  for (const path of ['/', '/posts', '/clips', '/settings'])
    expect(routeLocation(path).ancestors).toEqual([])
  expect(currentDestination('/setup')).toBe('/')
  expect(routeLocation('/admin/models')).toMatchObject({
    current: 'adminModels',
    destination: undefined,
  })
  expect(routeLocation('/billing/checkout').ancestors.at(-1)?.href).toBe('/billing')
  expect(routeLocation('/missing')).toMatchObject({
    current: 'notFound',
    ancestors: [],
    destination: undefined,
  })
})

// The retired hubs hand over to the places that took their jobs.
it.each([
  ['/my', '/posts'],
  ['/style', '/'],
  ['/create', '/'],
  ['/library', '/posts'],
  ['/insta-clip', '/clips'],
])('replaces the earlier %s with %s', async (from, to) => {
  const view = renderAppAt(from, { user: { id: 'alice' } })
  await waitFor(() => expect(view.router.state.location.pathname).toBe(to))
})

it.each(['/video-templates/new', '/video-templates/saved'])(
  'returns to the video template list from %s',
  (path) => {
    expect(routeLocation(path).ancestors.at(-1)?.href).toBe('/video-templates')
  },
)
