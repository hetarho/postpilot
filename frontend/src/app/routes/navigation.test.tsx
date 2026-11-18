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
// Guide: four places. 홈 holds the writing kit and every screen that makes it ready; 글 and 영상
// hold the posts and the videos (영상 also what makes a video); AI 토너먼트 the comparisons and the
// AI in use. Account, plan and settings belong to no tab: they live behind the avatar.
it.each([
  ['/', '/'],
  ['/voices', '/'],
  ['/voices/example/materials', '/'],
  ['/templates', '/'],
  ['/templates/new', '/'],
  ['/guidelines', '/'],
  ['/memories', '/'],
  ['/posts', '/posts'],
  ['/posts/new', '/posts'],
  ['/posts/example', '/posts'],
  ['/clips', '/clips'],
  ['/clips/new', '/clips'],
  ['/clips/example', '/clips'],
  ['/video-templates', '/clips'],
  ['/video-templates/new', '/clips'],
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
it('places the kit screens under 홈 and every post and clip under its list', () => {
  for (const path of ['/voices', '/templates', '/guidelines', '/memories'])
    expect(routeLocation(path).ancestors.map((x) => x.href)).toEqual(['/'])
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
  // The current tab is told by the accent ink and aria-current, never by colour alone; its
  // highlight holds the icon and the name together as one block.
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
  // The top bar is one slim row: the brand, which opens 홈, and the avatar — no credits, no
  // theme, no language at any width; those live behind the avatar.
  const banner = screen.getByRole('banner')
  expect(banner.children).toHaveLength(1)
  expect(banner).toContainElement(screen.getByRole('button', { name: '내 계정' }))
  expect(banner.querySelector('a[href="/"]')).not.toBeNull()
  expect(within(banner).queryByRole('button', { name: '테마' })).toBeNull()
  expect(within(banner).queryByRole('link', { name: /크레딧/ })).toBeNull()
  expect(banner).not.toContainElement(nav)

  await user.click(within(nav).getByRole('link', { name: '홈' }))
  await screen.findByRole('heading', { name: /고품질의 글을/, level: 1 })
  expect(view.router.state.location.pathname).toBe('/')
  expect(within(nav).getByRole('link', { name: '홈' })).toHaveAttribute('aria-current', 'page')

  // The writing screen hides the bar; the avatar still reaches 설정, which no tab holds.
  await act(() => view.router.navigate({ to: '/posts/new' }))
  await screen.findByRole('heading', { name: '어떤 글을 쓰시나요?', level: 2 })
  expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
  await user.click(screen.getByRole('button', { name: '내 계정' }))
  const panel = within(screen.getByRole('dialog', { name: '내 계정' }))
  // The theme and the language are inside the avatar, at every width.
  expect(panel.getByRole('group', { name: '인터페이스 환경설정' })).toBeInTheDocument()
  await user.click(panel.getByRole('link', { name: '설정' }))
  await screen.findByRole('heading', { name: '설정', level: 1 })
  const bar = screen.getByRole('navigation', { name: '주요' })
  expect(bar.querySelector('[aria-current]')).toBeNull()
  expect(calls.filter((call) => /^(Start|Create|Generate|Cancel)/.test(call))).toEqual([])
})
// Guide: 글 and 영상 each open on their title and one line saying what they hold; 영상's chips
// are the videos and what makes a video. Neither has a back link.
it("opens 글 and 영상 on what they hold, with 영상's kit as chips", async () => {
  viewport(false)
  const view = renderAppAt('/clips', { user: { id: 'alice' } })
  const chips = within(await screen.findByRole('navigation', { name: '나누어 보기' }))
  expect(chips.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '영상',
    '영상 템플릿',
    '영상 지침',
    '목소리',
  ])
  expect(chips.getByRole('link', { name: '영상' })).toHaveAttribute('aria-current', 'page')
  expect(screen.getByText('쇼츠와 릴스로 올릴 세로 영상')).toBeInTheDocument()
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()

  await act(() => view.router.navigate({ to: '/posts' }))
  expect(await screen.findByText('네이버 블로그에 올릴 내 글')).toBeInTheDocument()
  expect(screen.queryByRole('navigation', { name: '나누어 보기' })).toBeNull()
})
// Guide: a kit screen is a child of 홈 — it lights 홈 and has one way back to it.
it('opens each kit screen under 홈 with one way back to it', async () => {
  viewport(true)
  renderAppAt('/memories', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '기억', level: 1 })
  const nav = screen.getByRole('navigation', { name: '주요' })
  expect(within(nav).getByRole('link', { name: '홈' })).toHaveAttribute('aria-current', 'page')
  // The way back starts the title's row, and the shell draws none of its own.
  const back = within(screen.getByRole('main')).getByRole('link', { name: '홈' })
  expect(back).toHaveAttribute('href', '/')
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()
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
// 홈 draws the kit as one picture: the pen in its middle is 글쓰기, a piece with an account default
// opens it in a sheet whose foot leads to the piece's screen, a piece with none opens its screen,
// and 영상 만들기 is the quiet second way in. Looking at it writes nothing.
it.each([
  ['ko', '글쓰기', '영상 만들기', /^말투/, /^템플릿/, '말투 화면으로 가기'],
  ['en', 'Write', 'Make a video', /^Writing style/, /^Template/, 'Go to writing styles'],
] as const)(
  'starts a post from the kit on 홈 in %s without object writes',
  async (locale, write, video, voice, template, voiceScreen) => {
    viewport(true)
    initializeI18n(locale)
    const calls: string[] = []
    renderAppAt('/', { user: { id: 'alice' }, calls })
    const main = await screen.findByRole('main')
    expect(await within(main).findByRole('link', { name: write })).toHaveAttribute(
      'href',
      '/posts/new',
    )
    expect(within(main).getByRole('link', { name: video })).toHaveAttribute('href', '/clips/new')
    expect(within(main).getAllByRole('link', { name: template })[0]).toHaveAttribute(
      'href',
      '/templates',
    )
    await userEvent.click(within(main).getByRole('button', { name: voice }))
    const sheet = within(await screen.findByRole('dialog'))
    expect(sheet.getByRole('link', { name: voiceScreen })).toHaveAttribute('href', '/voices')
    expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
  },
)
// The retired tab addresses land somewhere that holds what they used to.
it.each([
  ['/create', '/'],
  ['/style', '/'],
  ['/my', '/posts'],
  ['/library', '/posts'],
] as const)('replaces %s with %s', async (from, to) => {
  const { router } = renderAppAt(from, { user: { id: 'alice' } })
  await waitFor(() => expect(router.state.location.pathname).toBe(to))
})

it('derives named child locations and unknown-route behavior from the same hierarchy', () => {
  for (const path of ['/', '/posts', '/clips']) expect(routeLocation(path).ancestors).toEqual([])
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

it('keeps the 영상 nodes, chip scroll and selected section through every sibling page', async () => {
  const view = renderAppAt('/clips', { user: { id: 'alice' } })
  const sections = await screen.findByRole('navigation', { name: '나누어 보기' })
  sections.scrollLeft = 24
  for (const [label, path] of [
    ['영상 지침', '/video-guidelines'],
    ['목소리', '/spoken-voices'],
    ['영상', '/clips'],
  ]) {
    await userEvent.click(within(sections).getByRole('link', { name: label }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(path))
    expect(screen.getByRole('navigation', { name: '나누어 보기' })).toBe(sections)
    expect(sections.scrollLeft).toBe(24)
    expect(within(sections).getByRole('link', { name: label })).toHaveAttribute(
      'aria-current',
      'page',
    )
    expect(sections.querySelectorAll('[aria-current]')).toHaveLength(1)
    expect(readNavigationEntry('alice', path)).toBeUndefined()
  }
  // The old Instagram list is not a section of its own: its address lands on 영상, inside the
  // same hub.
  await act(() => view.router.navigate({ to: '/insta-clip' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/clips'))
  expect(screen.getByRole('navigation', { name: '나누어 보기' })).toBe(sections)
})

it.each(['/video-templates/new', '/video-templates/saved'])(
  'returns to the video template list from %s',
  (path) => {
    expect(routeLocation(path).ancestors.at(-1)?.href).toBe('/video-templates')
  },
)
