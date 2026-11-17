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
// Four places: 홈 holds the writing kit (말투 · 템플릿 · 지침 · 기억), 글 every post, 영상 every
// video with what shapes one (영상 템플릿 · 영상 지침 · 목소리), AI 토너먼트 the AIs compared. The
// account and its settings are behind the avatar, which no tab marks.
it.each([
  ['/', '/'],
  ['/posts', '/posts'],
  ['/posts/new', '/posts'],
  ['/posts/example', '/posts'],
  ['/clips', '/clips'],
  ['/clips/new', '/clips'],
  ['/clips/example', '/clips'],
  ['/voices', '/'],
  ['/voices/example/materials', '/'],
  ['/templates', '/'],
  ['/templates/new', '/'],
  ['/guidelines', '/'],
  ['/memories', '/'],
  ['/video-templates', '/clips'],
  ['/video-guidelines', '/clips'],
  ['/spoken-voices', '/clips'],
  ['/spoken-voices/new', '/clips'],
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
  // The post list is the 글 tab: current by the accent ink and aria-current, never by colour
  // alone, its highlight holding the icon and the name together.
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
  // The top bar is one slim row: the brand (home) and the avatar. Theme and language are inside
  // the avatar at every width, never beside it.
  const banner = screen.getByRole('banner')
  expect(banner.children).toHaveLength(1)
  expect(banner).toContainElement(screen.getByRole('button', { name: '내 계정' }))
  expect(banner.querySelector('a[href="/"]')).not.toBeNull()
  expect(within(banner).queryByRole('group', { name: '인터페이스 환경설정' })).toBeNull()
  expect(banner).not.toContainElement(nav)
  await user.click(screen.getByRole('button', { name: '내 계정' }))
  const account = screen.getByRole('dialog', { name: '내 계정' })
  expect(within(account).getByRole('group', { name: '인터페이스 환경설정' })).toBeInTheDocument()
  expect(within(account).getByRole('link', { name: '내 설정' })).toHaveAttribute(
    'href',
    '/settings',
  )
  await user.keyboard('{Escape}')

  await user.click(within(nav).getByRole('link', { name: '홈' }))
  await screen.findByRole('heading', { name: /고품질의 글을/, level: 1 })
  expect(view.router.state.location.pathname).toBe('/')
  expect(within(nav).getByRole('link', { name: '홈' })).toHaveAttribute('aria-current', 'page')
  // The writing screen hides the bar; the avatar still reaches the settings, which no tab marks.
  await act(() => view.router.navigate({ to: '/posts/new' }))
  await screen.findByRole('heading', { name: '어떤 글을 쓰시나요?', level: 2 })
  expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
  await user.click(screen.getByRole('button', { name: '내 계정' }))
  await user.click(
    within(screen.getByRole('dialog', { name: '내 계정' })).getByRole('link', {
      name: '내 설정',
    }),
  )
  await screen.findByRole('heading', { name: '설정', level: 1 })
  expect(
    screen.getByRole('navigation', { name: '주요' }).querySelector('[aria-current]'),
  ).toBeNull()
  expect(calls.filter((call) => /^(Start|Create|Generate|Cancel)/.test(call))).toEqual([])
})
// Guide: a hub's pages share a large title and a chip row: the writing kit's four elements (each
// chip carrying its element's ring), and 영상 with what shapes a video.
it('opens hub pages on the hub title and their sibling chips, with no back link', async () => {
  viewport(false)
  const view = renderAppAt('/clips', { user: { id: 'alice' } })
  const chips = within(await screen.findByRole('navigation', { name: '나누어 보기' }))
  expect(chips.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '내 영상',
    '템플릿',
    '지침',
    '목소리',
  ])
  expect(chips.getByRole('link', { name: '내 영상' })).toHaveAttribute('aria-current', 'page')
  expect(chips.getByRole('link', { name: '템플릿' })).toHaveAttribute('href', '/video-templates')
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()

  await act(() => view.router.navigate({ to: '/templates' }))
  const kitChips = within(await screen.findByRole('navigation', { name: '나누어 보기' }))
  await waitFor(() =>
    expect(kitChips.getAllByRole('link').map((link) => link.textContent)).toEqual([
      '말투',
      '템플릿',
      '지침',
      '기억',
    ]),
  )
  expect(kitChips.getByRole('link', { name: '템플릿' })).toHaveAttribute('aria-current', 'page')
  expect(screen.queryByRole('navigation', { name: '글 · 영상' })).toBeNull()
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '홈' }),
  ).toHaveAttribute('aria-current', 'page')
})
it('gives a 설정 child one way back to 설정, and remembers where it was opened from', async () => {
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
  await act(() => router.navigate({ to: '/settings' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  // Settings is the avatar's destination, not a tab.
  expect(
    screen.getByRole('navigation', { name: '주요' }).querySelector('[aria-current]'),
  ).toBeNull()
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
// 홈 is the writing kit: one way into each element and the one 글쓰기, in both languages.
it.each([
  ['ko', '글쓰기', ['말투', '템플릿', '지침', '기억']],
  ['en', 'Write', ['Style', 'Templates', 'Rules', 'Memories']],
] as const)(
  'opens home on the kit and its one 글쓰기 in %s without object writes',
  async (locale, write, names) => {
    viewport(true)
    initializeI18n(locale)
    const calls: string[] = []
    renderAppAt('/', { user: { id: 'alice' }, calls })
    const main = await screen.findByRole('main')
    const kit = within(await within(main).findByRole('figure'))
    expect(
      names.map((name) =>
        kit.getByRole('link', { name: new RegExp('^' + name) }).getAttribute('href'),
      ),
    ).toEqual(['/voices', '/templates', '/guidelines', '/memories'])
    expect(within(main).getByRole('link', { name: write })).toHaveAttribute('href', '/posts/new')
    // Writing a video starts from the 영상 tab, not here.
    expect(main.querySelector('a[href="/clips/new"]')).toBeNull()
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

it('keeps the 영상 hub nodes, chip scroll and selected section through every sibling page', async () => {
  const view = renderAppAt('/clips', { user: { id: 'alice' } })
  const sections = await screen.findByRole('navigation', { name: '나누어 보기' })
  const title = sections.previousElementSibling
  sections.scrollLeft = 24
  for (const [label, path] of [
    ['목소리', '/spoken-voices'],
    ['지침', '/video-guidelines'],
    ['내 영상', '/clips'],
  ]) {
    await userEvent.click(within(sections).getByRole('link', { name: label }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(path))
    expect(screen.getByRole('navigation', { name: '나누어 보기' })).toBe(sections)
    expect(sections.previousElementSibling).toBe(title)
    expect(sections.scrollLeft).toBe(24)
    expect(within(sections).getByRole('link', { name: label })).toHaveAttribute(
      'aria-current',
      'page',
    )
    expect(sections.querySelectorAll('[aria-current]')).toHaveLength(1)
    expect(readNavigationEntry('alice', path)).toBeUndefined()
  }
  // The old Instagram list lands on 내 영상, inside the same hub.
  await act(() => view.router.navigate({ to: '/insta-clip' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/clips'))
  expect(screen.getByRole('navigation', { name: '나누어 보기' })).toBe(sections)
  expect(within(sections).getByRole('link', { name: '내 영상' })).toHaveAttribute(
    'aria-current',
    'page',
  )
})

it.each(['/video-templates/new', '/video-templates/saved'])(
  'returns to the video template list from %s',
  (path) => {
    expect(routeLocation(path).ancestors.at(-1)?.href).toBe('/video-templates')
  },
)
