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
// Guide: four places. 홈 holds the writing kit (its screens light it), 글 and 영상 hold the work,
// AI 토너먼트 picks and compares models; the account lives behind the avatar, in no tab.
it.each([
  ['/', '/'],
  ['/posts/new', '/posts'],
  ['/clips/new', '/clips'],
  ['/posts', '/posts'],
  ['/clips', '/clips'],
  ['/posts/example', '/posts'],
  ['/clips/example', '/clips'],
  ['/voices', '/'],
  ['/voices/example/materials', '/'],
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
it('places every post and clip under its channel list and exposes deterministic hierarchy', () => {
  expect(routeLocation('/posts/minted').ancestors.map((x) => x.href)).toEqual(['/posts'])
  expect(routeLocation('/clips/minted').ancestors.map((x) => x.href)).toEqual(['/clips'])
  expect(routeLocation('/voices/voice/materials').ancestors.map((x) => x.href)).toEqual([
    '/voices',
    '/voices/voice',
  ])
  expect(routeLocation('/voices/voice').ancestors.map((x) => x.href)).toEqual(['/voices'])
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
  // The 글 list is its own tab: current, told by the accent ink and aria-current, never by colour
  // alone, its highlight holding the icon and the name together as one block.
  const posts = within(nav).getByRole('link', { name: '글' })
  expect(posts).toHaveAttribute('aria-current', 'page')
  expect(posts).toHaveClass('text-tab-bar-fg-current')
  const indicator = posts.querySelector('.bg-tab-bar-indicator')
  expect(indicator).not.toBeNull()
  expect(indicator).toContainElement(posts.querySelector('svg'))
  for (const name of ['홈', '영상', 'AI 토너먼트']) {
    const link = within(nav).getByRole('link', { name })
    expect(link).not.toHaveAttribute('aria-current')
    expect(link.querySelector('.bg-tab-bar-indicator')).toBeNull()
  }
  // The 글 tab says what it holds under its title.
  expect(screen.getByText('블로그 글 — 쓰고 있는 글과 다 쓴 글')).toBeInTheDocument()
  // The top bar is one slim row: the brand, which goes home, and the avatar. No destinations and
  // no separate theme or language control: both live inside the avatar.
  const banner = screen.getByRole('banner')
  expect(banner.children).toHaveLength(1)
  expect(banner).toContainElement(screen.getByRole('button', { name: '내 계정' }))
  expect(banner.querySelector('a[href="/"]')).not.toBeNull()
  expect(within(banner).queryByRole('group', { name: '인터페이스 환경설정' })).toBeNull()
  expect(banner).not.toContainElement(nav)

  await user.click(within(nav).getByRole('link', { name: '홈' }))
  await screen.findByRole('heading', { level: 1, name: /고품질의 글을/ })
  expect(view.router.state.location.pathname).toBe('/')
  expect(within(nav).getByRole('link', { name: '홈' })).toHaveAttribute('aria-current', 'page')
  // Home starts a post: its one creation action opens the editor.
  expect(screen.getByRole('link', { name: '글쓰기' })).toHaveAttribute('href', '/posts/new')

  // Settings, the plan and the interface preferences are behind the avatar.
  await user.click(screen.getByRole('button', { name: '내 계정' }))
  const panel = screen.getByRole('dialog', { name: '내 계정' })
  expect(within(panel).getByRole('group', { name: '인터페이스 환경설정' })).toBeInTheDocument()
  await user.click(within(panel).getByRole('link', { name: '설정' }))
  await screen.findByRole('heading', { name: '설정', level: 1 })
  expect(
    screen.getByRole('navigation', { name: '주요' }).querySelector('[aria-current]'),
  ).toBeNull()

  // The writing screen hides the bar. A new draft opens on the guided flow's first question.
  await act(() => view.router.navigate({ to: '/posts/new' }))
  await screen.findByRole('heading', { name: '어떤 글을 쓰시나요?', level: 2 })
  expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
  expect(calls.filter((call) => /^(Start|Create|Generate|Cancel)/.test(call))).toEqual([])
})
// Guide: the kit's pages share a large title and a chip row; 템플릿 and 지침 add a 글 · 영상 switch.
// The 글 and 영상 tabs are single lists with no chip row.
it('opens kit pages on the kit title and their sibling chips, with no back link', async () => {
  viewport(false)
  const view = renderAppAt('/clips', { user: { id: 'alice' } })
  await screen.findByText('숏폼 영상 — 만들고 있는 영상과 완성된 영상')
  expect(screen.queryByRole('navigation', { name: '나누어 보기' })).toBeNull()
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()

  await act(() => view.router.navigate({ to: '/templates' }))
  const styleChips = within(await screen.findByRole('navigation', { name: '나누어 보기' }))
  await waitFor(() =>
    expect(styleChips.getAllByRole('link').map((link) => link.textContent)).toEqual([
      '말투',
      '템플릿',
      '지침',
      '기억',
      '목소리',
    ]),
  )
  expect(screen.getByText('내 스타일')).toBeInTheDocument()
  expect(styleChips.getByRole('link', { name: '템플릿' })).toHaveAttribute('aria-current', 'page')
  const kinds = within(screen.getByRole('navigation', { name: '글 · 영상' }))
  expect(kinds.getByRole('link', { name: '글' })).toHaveAttribute('aria-current', 'page')
  expect(kinds.getByRole('link', { name: '영상' })).toHaveAttribute('href', '/video-templates')
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()
  // The kit is what home draws, so its pages light 홈.
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '홈' }),
  ).toHaveAttribute('aria-current', 'page')
})
it('gives a settings child one way back to 설정, and remembers where it was opened from', async () => {
  viewport(true)
  const { router } = renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '설정', level: 1 })
  await act(() => router.navigate({ to: '/account' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/account'))
  // The page's frame draws the way back in its own column, on the title's row, and the shell
  // draws none of its own: one way back, not two.
  const page = await screen.findByRole('main')
  const back = within(page).getByRole('link', { name: '설정' })
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
  await screen.findByRole('heading', { name: '설정', level: 1 })
  expect(screen.queryByRole('link', { name: '관리자' })).toBeNull()
})
it('retains a 설정 origin through reload and restores its document position on explicit return', async () => {
  viewport(true)
  const calls: string[] = []
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(window, 'scrollY', 'get').mockReturnValue(480)
  vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(2400)
  const view = renderAppAt('/settings', { user: { id: 'alice' }, calls })
  await screen.findByRole('heading', { name: '설정', level: 1 })
  await act(() => view.router.navigate({ to: '/account' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/account'))
  expect(readNavigationEntry('alice', '/account')).toMatchObject({
    path: '/settings',
    scrollY: 480,
  })
  view.unmount()
  const reopened = renderAppAt('/account', { transport: view.transport })
  await screen.findByRole('heading', { level: 1, name: '계정 설정' })
  await userEvent.click(within(screen.getByRole('main')).getByRole('link', { name: '설정' }))
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/settings'))
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 480))
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
// Home draws the kit from the lists the kit's screens read, and holds one creation entry.
it.each([
  ['ko', '글쓰기', '템플릿', '내 스타일'],
  ['en', 'Write', 'Template', 'My style'],
] as const)(
  'draws the writing kit with one creation entry in %s without object writes',
  async (locale, write, template, legend) => {
    viewport(true)
    initializeI18n(locale)
    const calls: string[] = []
    renderAppAt('/', { user: { id: 'alice' }, calls })
    const main = await screen.findByRole('main')
    expect(await within(main).findByRole('link', { name: write })).toHaveAttribute(
      'href',
      '/posts/new',
    )
    // The kit is drawn once as the post it writes (one image) and keyed by a legend whose four
    // rows open the four screens.
    expect(within(main).getByRole('img', { name: new RegExp(template) })).toBeInTheDocument()
    const kit = within(within(main).getByRole('navigation', { name: legend }))
    expect(kit.getAllByRole('link')).toHaveLength(4)
    expect(kit.getByRole('link', { name: new RegExp('^' + template) })).toBeInTheDocument()
    expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
  },
)

it('derives named child locations and unknown-route behavior from the same hierarchy', () => {
  for (const path of ['/', '/posts', '/clips', '/voices'])
    expect(routeLocation(path).ancestors).toEqual([])
  // The setup that fills several kit layers at once is opened from home.
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

it('sends the retired tab addresses to the place that does their job now', async () => {
  const view = renderAppAt('/my', { user: { id: 'alice' } })
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/posts'))
  for (const [from, to] of [
    ['/style', '/'],
    ['/create', '/'],
    ['/insta-clip', '/clips'],
  ] as const) {
    await act(() => view.router.navigate({ to: from }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(to))
  }
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '영상' }),
  ).toHaveAttribute('aria-current', 'page')
})

it.each(['/video-templates/new', '/video-templates/saved'])(
  'returns to the video template list from %s',
  (path) => {
    expect(routeLocation(path).ancestors.at(-1)?.href).toBe('/video-templates')
  },
)
