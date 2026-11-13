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
// Guide: five verb tabs. Every post and clip lives in 내 글, everything taught to the AI in
// 내 스타일, model choice and comparison in AI 고르기, and the account in 나.
it.each([
  ['/', undefined],
  ['/create', '/create'],
  ['/my', '/my'],
  ['/posts/new', '/my'],
  ['/clips/new', '/my'],
  ['/clips/new?channel=instagram_clip', '/my'],
  ['/naver-blog', '/my'],
  ['/insta-clip', '/my'],
  ['/posts/example', '/my'],
  ['/clips/example', '/my'],
  ['/style', '/style'],
  ['/voices', '/style'],
  ['/voices/example/materials', '/style'],
  ['/templates/new', '/style'],
  ['/video-templates?channel=naver_clip', '/style'],
  ['/guidelines', '/style'],
  ['/memories', '/style'],
  ['/spoken-voices/new', '/style'],
  ['/ai-models', '/ai-models'],
  ['/tests', '/ai-models'],
  ['/tests/history', '/ai-models'],
  ['/tests/test', '/ai-models'],
  ['/ai-models/compare', '/ai-models'],
  ['/billing', '/settings'],
  ['/plans', '/settings'],
  ['/admin/models', '/settings'],
] as const)('resolves %s to %s', (path, tab) => expect(currentDestination(path)).toBe(tab))
it('places every post and clip under its channel list and exposes deterministic hierarchy', () => {
  expect(routeLocation('/posts/minted').ancestors.map((x) => x.href)).toEqual(['/naver-blog'])
  expect(routeLocation('/clips/minted', 'instagram_clip').ancestors.map((x) => x.href)).toEqual([
    '/insta-clip',
  ])
  expect(routeLocation('/voices/voice/materials').ancestors.map((x) => x.href)).toEqual([
    '/voices',
    '/voices/voice',
  ])
  expect(routeLocation('/voices/voice', 'instagram_clip').ancestors.map((x) => x.href)).toEqual([
    '/voices?channel=instagram_clip',
  ])
  expect(routeLocation('/tests/id').ancestors.map((x) => x.href)).toEqual(['/ai-models', '/tests'])
  expect(routeLocation('/tests/records/id').ancestors.map((x) => x.href)).toEqual([
    '/ai-models',
    '/tests/history',
  ])
})
// Guide: the destinations are a bottom tab bar at every width, a floating pill on the desk. The
// writing screen is a focused flow: it hides the bar and keeps its own way back.
it.each([false, true])('keeps the five verbs one tap away, desktop=%s', async (desktop) => {
  viewport(desktop)
  const user = userEvent.setup()
  const calls: string[] = []
  const view = renderAppAt('/naver-blog', { user: { id: 'alice', plan: ProtoPlan.FREE }, calls })
  const nav = await screen.findByRole('navigation', { name: '주요' })
  const links = within(nav).getAllByRole('link')
  expect(links.map((link) => link.textContent)).toEqual([
    '만들기',
    '내 글',
    '내 스타일',
    'AI 고르기',
    '나',
  ])
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
    '/create',
    '/my',
    '/style',
    '/ai-models',
    '/settings',
  ])
  // A channel's list is a page of 내 글: that tab is current, told by the accent ink and
  // aria-current, never by colour alone.
  const mine = within(nav).getByRole('link', { name: '내 글' })
  expect(mine).toHaveAttribute('aria-current', 'page')
  expect(mine).toHaveClass('text-tab-bar-fg-current')
  for (const name of ['만들기', '내 스타일', 'AI 고르기', '나']) {
    const link = within(nav).getByRole('link', { name })
    expect(link).not.toHaveAttribute('aria-current')
    expect(link).toHaveClass('text-tab-bar-fg')
  }
  // The top bar is one slim row: the brand and the account menu, no destinations.
  const banner = screen.getByRole('banner')
  expect(banner.children).toHaveLength(1)
  expect(banner).toContainElement(screen.getByRole('button', { name: '내 계정' }))
  expect(banner.querySelector('a[href="/create"]')).not.toBeNull()
  expect(banner).not.toContainElement(nav)
  expect(screen.queryByRole('button', { name: '이동 메뉴' })).toBeNull()

  await user.click(within(nav).getByRole('link', { name: '내 스타일' }))
  await screen.findByRole('heading', { name: '내 스타일', level: 1 })
  expect(view.router.state.location.pathname).toBe('/style')
  expect(within(nav).getByRole('link', { name: '내 스타일' })).toHaveAttribute(
    'aria-current',
    'page',
  )
  await user.click(within(nav).getByRole('link', { name: '나' }))
  await screen.findByRole('heading', { name: '나', level: 1 })
  expect(within(nav).getByRole('link', { name: '나' })).toHaveAttribute('aria-current', 'page')
  await user.click(within(nav).getByRole('link', { name: '만들기' }))
  await screen.findByRole('heading', { name: '무엇을 만들까요?', level: 1 })
  // 만들기 asks one question: three big tiles, each opening its guided flow.
  const choices = within(screen.getByRole('navigation', { name: '만들 것' }))
  expect(choices.getAllByRole('link').map((link) => link.getAttribute('href'))).toEqual([
    '/posts/new',
    '/clips/new?channel=naver_clip',
    '/clips/new?channel=instagram_clip',
  ])
  // The writing screen hides the bar; the account menu still reaches 나. A new draft opens on
  // the guided flow's first question, 글 종류.
  await act(() => view.router.navigate({ to: '/posts/new' }))
  await screen.findByRole('heading', { name: '어떤 글을 쓰시나요?', level: 2 })
  expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
  await user.click(screen.getByRole('button', { name: '내 계정' }))
  await user.click(
    within(screen.getByRole('dialog', { name: '내 계정' })).getByRole('link', {
      name: '내 설정',
    }),
  )
  await screen.findByRole('heading', { name: '나', level: 1 })
  expect(screen.getByRole('navigation', { name: '주요' })).toBeInTheDocument()
  expect(calls.filter((call) => /^(Start|Create|Generate|Cancel)/.test(call))).toEqual([])
})
// Guide: a hub's pages share a large title and a chip row; 템플릿 and 지침 add a 글 · 영상 switch.
it('opens hub pages on the hub title and their sibling chips, with no back link', async () => {
  viewport(false)
  const view = renderAppAt('/naver-clip', { user: { id: 'alice' } })
  const chips = within(await screen.findByRole('navigation', { name: '나누어 보기' }))
  expect(chips.getAllByRole('link').map((link) => link.textContent)).toEqual([
    '전체',
    '블로그',
    '네이버 클립',
    '인스타 클립',
  ])
  expect(chips.getByRole('link', { name: '네이버 클립' })).toHaveAttribute('aria-current', 'page')
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
  expect(styleChips.getByRole('link', { name: '템플릿' })).toHaveAttribute('aria-current', 'page')
  const kinds = within(screen.getByRole('navigation', { name: '글 · 영상' }))
  expect(kinds.getByRole('link', { name: '글' })).toHaveAttribute('aria-current', 'page')
  expect(kinds.getByRole('link', { name: '영상' })).toHaveAttribute(
    'href',
    '/video-templates?channel=naver_clip',
  )
  expect(screen.queryByRole('navigation', { name: '돌아가기' })).toBeNull()
})
it('gives a 나 child one way back to 나, and remembers where it was opened from', async () => {
  viewport(true)
  const { router } = renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '나', level: 1 })
  await act(() => router.navigate({ to: '/account' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/account'))
  const back = within(screen.getByRole('navigation', { name: '돌아가기' })).getByRole('link')
  expect(back).toHaveAccessibleName('나')
  expect(back).toHaveAttribute('href', '/settings')
  expect(screen.queryByRole('navigation', { name: '현재 위치' })).toBeNull()
  await act(() => router.navigate({ to: '/settings' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/settings'))
  expect(
    screen.getByRole('navigation', { name: '주요' }).querySelector('[aria-current]'),
  ).toHaveTextContent('나')
})
it('keeps administration protected and discoverable only to its owner role', async () => {
  viewport(true)
  renderAppAt('/settings', { user: { id: 'operator', plan: ProtoPlan.MASTER } })
  expect(await screen.findByRole('link', { name: '관리자' })).toHaveAttribute('href', '/admin')
  cleanup()
  renderAppAt('/settings', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '나', level: 1 })
  expect(screen.queryByRole('link', { name: '관리자' })).toBeNull()
})
it('retains a 나 origin through reload and restores its document position on explicit return', async () => {
  viewport(true)
  const calls: string[] = []
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(window, 'scrollY', 'get').mockReturnValue(480)
  vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(2400)
  const view = renderAppAt('/settings', { user: { id: 'alice' }, calls })
  await screen.findByRole('heading', { name: '나', level: 1 })
  await act(() => view.router.navigate({ to: '/account' }))
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/account'))
  expect(readNavigationEntry('alice', '/account')).toMatchObject({
    path: '/settings',
    scrollY: 480,
  })
  view.unmount()
  const reopened = renderAppAt('/account', { transport: view.transport })
  await userEvent.click(
    within(await screen.findByRole('navigation', { name: '돌아가기' })).getByRole('link', {
      name: '나',
    }),
  )
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/settings'))
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 480))
  expect(calls.some((x) => /^(Start|Save|Cancel|Create)/.test(x))).toBe(false)
})
it.each([
  ['ko', '블로그 글', '네이버 클립', '인스타 클립'],
  ['en', 'Blog post', 'Naver Clip', 'Instagram Clip'],
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
  for (const path of ['/create', '/my', '/style', '/naver-blog', '/naver-clip', '/insta-clip'])
    expect(routeLocation(path).ancestors).toEqual([])
  expect(currentDestination('/setup')).toBeUndefined()
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

it('keeps the My work hub nodes, chip scroll and selected section through every sibling list', async () => {
  const view = renderAppAt('/my', { user: { id: 'alice' } })
  const sections = await screen.findByRole('navigation', { name: '나누어 보기' })
  const title = sections.previousElementSibling
  sections.scrollLeft = 24
  for (const [label, path, heading] of [
    ['블로그', '/naver-blog', '네이버 블로그'],
    ['네이버 클립', '/naver-clip', '네이버 클립'],
    ['인스타 클립', '/insta-clip', '인스타 클립'],
    ['전체', '/my', '내 글'],
  ]) {
    await userEvent.click(within(sections).getByRole('link', { name: label }))
    await screen.findByRole('heading', { level: 1, name: heading })
    expect(view.router.state.location.pathname).toBe(path)
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
})

it.each(['/video-templates/new', '/video-templates/saved'])(
  'keeps a valid default or explicit channel in the return from %s',
  (path) => {
    expect(routeLocation(path).ancestors.at(-1)?.href).toBe('/video-templates?channel=naver_clip')
    expect(routeLocation(path, 'instagram_clip').ancestors.at(-1)?.href).toBe(
      '/video-templates?channel=instagram_clip',
    )
  },
)
