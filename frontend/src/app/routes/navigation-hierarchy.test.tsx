import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { paidActions, renderLegacyModelRoute } from '@/test/legacy-model-routes'
import { readNavigationEntry } from '../model/navigation-entry'

beforeEach(() => {
  sessionStorage.clear()
  initializeI18n('ko')
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('64rem'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => true,
    onchange: null,
  }))
})
afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  sessionStorage.clear()
})

/** The shell's one back link (THEME-58); a page with its own top-row return shows none. */
function backLink() {
  const region = screen.queryByRole('navigation', { name: '돌아가기' })
  return region ? within(region).getByRole('link') : undefined
}
function backHref() {
  return backLink()?.getAttribute('href') ?? undefined
}
/** A top-level destination: no location trail and no implicit way back. */
function expectRoot() {
  expect(screen.queryByRole('navigation', { name: '현재 위치' })).not.toBeInTheDocument()
  expect(backHref()).toBeUndefined()
}

/** Persist the old schema directly so new admission rules cannot hide a stale-cache regression. */
function legacyEntry(target: string, path: string, ownerKey = 'alice') {
  sessionStorage.setItem(
    `postpilot.return.alice.nav.${encodeURIComponent(target)}`,
    JSON.stringify({
      version: 1,
      ownerKey,
      targetId: target,
      path,
      section: '/library',
      filters: {},
      scrollY: 360,
    }),
  )
}

it('traverses channel hubs and settings as sibling destinations without paid work', async () => {
  const user = userEvent.setup()
  const view = renderLegacyModelRoute('/naver-blog', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '네이버 블로그', level: 1 })
  expectRoot()
  for (const [label, pathname, navLabel] of [
    ['네이버 클립', '/naver-clip', '클립'],
    ['인스타 클립', '/insta-clip', '인스타'],
    ['내 설정', '/settings', '내 설정'],
    ['네이버 블로그', '/naver-blog', '블로그'],
  ] as const) {
    const primary = within(screen.getByRole('navigation', { name: '주요' }))
    if (pathname === '/settings') {
      await user.click(screen.getByRole('button', { name: '내 계정' }))
      await user.click(
        within(screen.getByRole('dialog', { name: '내 계정' })).getByRole('link', {
          name: navLabel,
        }),
      )
    } else await user.click(primary.getByRole('link', { name: navLabel }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(pathname))
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot()
    if (pathname === '/settings')
      expect(
        primary.queryAllByRole('link').every((link) => !link.hasAttribute('aria-current')),
      ).toBe(true)
    else
      expect(primary.getByRole('link', { name: navLabel })).toHaveAttribute('aria-current', 'page')
  }
  expect(paidActions(view.procedures)).toEqual([])
})

it.each([
  ['/naver-clip', '네이버 클립'],
  ['/settings', '내 설정'],
  ['/naver-blog', '네이버 블로그'],
])(
  'ignores an old cross-destination origin on direct load and reload of %s',
  async (path, label) => {
    legacyEntry(path, '/posts')
    const view = renderLegacyModelRoute(path, { user: { id: 'alice' } })
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot()
    view.unmount()
    renderAppAt(path, { transport: view.transport })
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot()
    expect(paidActions(view.procedures)).toEqual([])
  },
)

it('does not make a generic entry parameter the parent of top-level writing tests', async () => {
  const view = renderLegacyModelRoute('/tests?entry=%2Flibrary', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expect(backHref()).toBeUndefined()
  expect(within(screen.getByRole('main')).queryByRole('link', { name: '작업 내역' })).toBeNull()
  expect(paidActions(view.procedures)).toEqual([])
})

it('returns a directly opened result to writing tests without fabricating a history ancestor', async () => {
  const view = renderLegacyModelRoute(
    '/tests/test',
    { user: { id: 'alice' } },
    { readableTest: true },
  )
  await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
  expect(backHref()).toBeUndefined()
  const back = within(screen.getByRole('main')).getByRole('link', { name: '글쓰기 테스트' })
  expect(back).toHaveAttribute('href', '/tests')
  await userEvent.click(back)
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/tests'))
  expect(backHref()).toBeUndefined()
  expect(paidActions(view.procedures)).toEqual([])
})

it.each([
  '/tests/history?stage=write&source=owned-source',
  '/tests/history?source=owned-source&stage=write',
])('preserves filtered history through a real click, reload and return from %s', async (entry) => {
  const history = '/tests/history?stage=write&source=owned-source'
  const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
  vi.spyOn(window, 'scrollY', 'get').mockReturnValue(360)
  vi.spyOn(document.documentElement, 'scrollHeight', 'get').mockReturnValue(2400)
  const view = renderLegacyModelRoute(entry, { user: { id: 'alice' } }, { readableTest: true })
  view.fixture.getTest()!.sourcePostSlug = 'owned-source'
  const record = await within(await screen.findByRole('main')).findByRole('link', {
    name: '테스트 이어보기',
  })
  const href = record.getAttribute('href')!
  await userEvent.click(record)
  await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
  expect(backHref()).toBeUndefined()
  expect(readNavigationEntry('alice', view.router.state.location.href)?.scrollY).toBe(360)
  view.unmount()
  const reopened = renderAppAt(href, { transport: view.transport })
  await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
  const back = within(screen.getByRole('main')).getByRole('link', { name: '테스트 기록' })
  expect(back).toHaveAttribute('href', history)
  await userEvent.click(back)
  await waitFor(() => expect(reopened.router.state.location.href).toBe(history))
  await waitFor(() => expect(scroll).toHaveBeenCalledWith(0, 360))
  expect(paidActions(view.procedures)).toEqual([])
})

it.each([
  ['alice', '/library'],
  ['bob', '/tests/history'],
])(
  'rejects a stale %s child origin from %s while retaining the named test parent',
  async (owner, path) => {
    legacyEntry('/tests/test', path, owner)
    const view = renderLegacyModelRoute(
      '/tests/test',
      { user: { id: 'alice' } },
      { readableTest: true },
    )
    await screen.findByRole('button', { name: '후보 A를 승자로 선택' })
    expect(backHref()).toBeUndefined()
    expect(
      within(screen.getByRole('main')).getByRole('link', { name: '글쓰기 테스트' }),
    ).toHaveAttribute('href', '/tests')
    expect(paidActions(view.procedures)).toEqual([])
  },
)

it('keeps an owned source-post return separate from top-level test breadcrumbs', async () => {
  const view = renderLegacyModelRoute(
    '/tests?source=owned-source&entry=%2Flibrary',
    { user: { id: 'alice' } },
    { source: true },
  )
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expect(backHref()).toBeUndefined()
  expect(within(screen.getByRole('main')).getByRole('link', { name: '글 작성' })).toHaveAttribute(
    'href',
    '/posts/owned-source',
  )
  expect(paidActions(view.procedures)).toEqual([])
})

it('does not offer a source-post return after the source is missing or inaccessible', async () => {
  const view = renderLegacyModelRoute(
    '/tests?source=missing-source&entry=%2Flibrary',
    { user: { id: 'alice' } },
    { source: true },
  )
  await screen.findByRole('alert')
  expect(backHref()).toBeUndefined()
  expect(within(screen.getByRole('main')).queryByRole('link', { name: '글 작성' })).toBeNull()
  expect(screen.queryByRole('link', { name: '돌아가기' })).toBeNull()
  expect(paidActions(view.procedures)).toEqual([])
})

it('does not propagate a sibling visit into a settings child origin', async () => {
  const view = renderLegacyModelRoute('/naver-blog', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '네이버 블로그', level: 1 })
  await act(() => view.router.navigate({ to: '/templates' }))
  await screen.findByRole('heading', { name: '템플릿', level: 1 })
  expect(backHref()).toBeUndefined()
  expect(readNavigationEntry('alice', '/templates')?.path).not.toBe('/library')
  expect(screen.queryByRole('link', { name: '돌아가기' })).toBeNull()
  expect(paidActions(view.procedures)).toEqual([])
})

it.each(['/ai-models/experiments/retained', '/posts/experiments/retained'])(
  'redirects %s to the canonical authenticated retained-record route without paid work',
  async (path) => {
    const calls: string[] = []
    const view = renderAppAt(path, {
      user: { id: 'alice' },
      calls,
      experiments: { history: [{ id: 'retained', stage: Stage.WRITE }] },
    })
    await screen.findByRole('heading', { name: '이전 유료 비교 기록', level: 1 })
    expect(view.router.state.location.pathname).toBe('/tests/records/retained')
    expect(backHref()).toBeUndefined()
    const primary = within(screen.getByRole('navigation', { name: '주요' }))
    expect(primary.queryAllByRole('link').every((link) => !link.hasAttribute('aria-current'))).toBe(
      true,
    )
    expect(
      within(screen.getByRole('main')).getByRole('link', { name: '테스트 기록' }),
    ).toHaveAttribute('href', '/tests/history')
    expect(paidActions(calls)).toEqual([])
  },
)

it.each([
  '/tests/records/retained',
  '/ai-models/experiments/retained',
  '/posts/experiments/retained',
])('authenticates %s before loading retained paid records', async (path) => {
  const calls: string[] = []
  const view = renderAppAt(path, { calls })
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/login'))
  expect(view.router.state.location.search.redirect).toBe(path)
  expect(calls).not.toContain('GetExperiment')
  expect(paidActions(calls)).toEqual([])
})

it.each([
  ['/naver-blog/', '네이버 블로그'],
  ['/settings/', '내 설정'],
  ['/insta-clip/', '인스타 클립'],
])('keeps trailing-slash root %s independent of explicit entry', async (path, label) => {
  const view = renderLegacyModelRoute(`${path}?entry=%2Fposts`, { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: label, level: 1 })
  expectRoot()
  expect(paidActions(view.procedures)).toEqual([])
})

it('retains the Instagram location through shared templates, child navigation, reload and return', async () => {
  const calls: string[] = []
  const view = renderAppAt('/insta-clip', { user: { id: 'alice' }, calls })
  await screen.findByRole('heading', { name: '인스타 클립', level: 1 })
  await userEvent.click(screen.getByRole('button', { name: '새 클립' }))
  await userEvent.click(screen.getByRole('menuitem', { name: '영상 템플릿' }))
  await screen.findByRole('heading', { name: '영상 템플릿', level: 1 })
  expect(backHref()).toBeUndefined()
  await userEvent.click(
    await within(screen.getByRole('main')).findByRole('link', { name: '새 영상 템플릿' }),
  )
  await screen.findByRole('heading', { name: '새 영상 템플릿', level: 1 })
  expect(backHref()).toBe('/video-templates?channel=instagram_clip')
  const address = view.router.state.location.href
  expect(address).toContain('channel=instagram_clip')
  view.unmount()
  sessionStorage.clear()
  const reopened = renderAppAt(address, { transport: view.transport })
  await screen.findByRole('heading', { name: '새 영상 템플릿', level: 1 })
  expect(backHref()).toBe('/video-templates?channel=instagram_clip')
  expect(backLink()).toHaveTextContent('영상 템플릿')
  await userEvent.click(backLink()!)
  // Back on a section list page: the section row marks it and there is no way back further.
  await waitFor(() =>
    expect(reopened.router.state.location.href).toBe('/video-templates?channel=instagram_clip'),
  )
  expect(backHref()).toBeUndefined()
  expect(paidActions(calls)).toEqual([])
})

it('keeps a directly opened Instagram project channel when visiting shared video guidelines', async () => {
  const calls: string[] = []
  const view = renderAppAt('/clips/owned', {
    user: { id: 'alice' },
    calls,
    clips: {
      projects: [
        {
          id: 'owned',
          title: '체험',
          channel: 'instagram_clip',
          videoTemplateId: '',
          ratio: 'vertical',
          targetDurationMs: 15000,
          disclosure: 'ad',
        },
      ],
    },
  })
  await screen.findByRole('link', { name: '인스타 클립으로 돌아가기' })
  await act(() => view.router.navigate({ to: '/video-guidelines' }))
  await screen.findByRole('heading', { name: '영상 지침', level: 1 })
  expect(backHref()).toBeUndefined()
  expect(paidActions(calls)).toEqual([])
})

it.each(['영상 템플릿', '영상 지침'])(
  'keeps Instagram when its menu %s item opens in a fresh tab',
  async (heading) => {
    const calls: string[] = []
    const view = renderAppAt('/insta-clip', { user: { id: 'alice' }, calls })
    await userEvent.click(await screen.findByRole('button', { name: '새 클립' }))
    const href = screen.getByRole('menuitem', { name: heading }).getAttribute('href')!
    expect(href).toContain('channel=instagram_clip')
    view.unmount()
    sessionStorage.clear()
    renderAppAt(href, { transport: view.transport })
    await screen.findByRole('heading', { name: heading, level: 1 })
    expect(backHref()).toBeUndefined()
    const nav = screen.getByRole('navigation', { name: '주요' })
    expect(within(nav).getByRole('link', { name: '인스타' })).toHaveAttribute(
      'aria-current',
      'page',
    )
    if (heading === '영상 템플릿') {
      expect(
        await within(screen.getByRole('main')).findByRole('link', { name: '새 영상 템플릿' }),
      ).toHaveAttribute('href', '/video-templates/new?channel=instagram_clip')
    }
    expect(paidActions(calls)).toEqual([])
  },
)

it.each([
  ['/templates', '블로그', '템플릿', ['/naver-blog', '/templates', '/voices', '/guidelines']],
  [
    '/voices?channel=instagram_clip',
    '인스타',
    '말투',
    [
      '/insta-clip',
      '/video-templates?channel=instagram_clip',
      '/voices?channel=instagram_clip',
      '/video-guidelines?channel=instagram_clip',
    ],
  ],
  ['/voices', '블로그', '말투', ['/naver-blog', '/templates', '/voices', '/guidelines']],
] as const)(
  'places the %s section under its channel as a sibling with no way back',
  async (path, channel, section, hrefs) => {
    const view = renderAppAt(path, { user: { id: 'alice' } })
    const sections = await screen.findByRole('navigation', { name: '채널 메뉴' })
    expect(
      within(sections)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual(hrefs)
    expect(within(sections).getByRole('link', { name: section })).toHaveAttribute(
      'aria-current',
      'page',
    )
    expect(
      within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', {
        name: channel,
      }),
    ).toHaveAttribute('aria-current', 'page')
    expect(backHref()).toBeUndefined()
    await userEvent.click(within(sections).getAllByRole('link')[0])
    await waitFor(() => expect(view.router.state.location.pathname).toBe(hrefs[0]))
    expect(readNavigationEntry('alice', hrefs[0])).toBeUndefined()
  },
)

it('returns a voice page opened from an Instagram 말투 list to that list', async () => {
  const view = renderAppAt('/voices?channel=instagram_clip', { user: { id: 'alice' } })
  await screen.findByRole('navigation', { name: '채널 메뉴' })
  await act(() =>
    view.router.navigate({ to: '/voices/$voiceId', params: { voiceId: 'voice-default' } }),
  )
  await waitFor(() => expect(backHref()).toBe('/voices?channel=instagram_clip'))
  expect(screen.queryByRole('navigation', { name: '채널 메뉴' })).toBeNull()
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '인스타' }),
  ).toHaveAttribute('aria-current', 'page')
})
