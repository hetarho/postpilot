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

/** The shell's one back link (THEME-58); a page with its own top-row return shows none, and a
 *  screen on a `PageFrame` draws it in its own column instead (THEME-75). */
function backLink() {
  const region = screen.queryByRole('navigation', { name: '돌아가기' })
  return region ? within(region).getByRole('link') : undefined
}
function backHref() {
  return backLink()?.getAttribute('href') ?? undefined
}
/** A framed screen's way back: the ← at the head of its own column, named by where it leads, and
 *  the only one — the shell then draws none. */
async function frameReturn(name: string) {
  const back = await screen.findByRole('link', { name })
  expect(back.querySelector('svg')).not.toBeNull()
  expect(backLink()).toBeUndefined()
  return back
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

it('traverses the four places as sibling destinations without paid work', async () => {
  const user = userEvent.setup()
  const view = renderLegacyModelRoute('/posts', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '네이버 블로그', level: 1 })
  expectRoot()
  // Guide: four places; the avatar reaches 설정, which lights no tab.
  for (const [label, pathname, navLabel, viaAccount] of [
    [/고품질의 글을/, '/', '홈', false],
    ['영상', '/clips', '영상', false],
    ['AI 토너먼트', '/ai-models', 'AI 토너먼트', false],
    ['설정', '/settings', undefined, true],
    ['네이버 블로그', '/posts', '글', false],
  ] as const) {
    const primary = within(screen.getByRole('navigation', { name: '주요' }))
    if (viaAccount) {
      await user.click(screen.getByRole('button', { name: '내 계정' }))
      await user.click(
        within(screen.getByRole('dialog', { name: '내 계정' })).getByRole('link', {
          name: '내 설정',
        }),
      )
    } else if (navLabel) await user.click(primary.getByRole('link', { name: navLabel }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(pathname))
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot()
    const current = primary
      .queryAllByRole('link')
      .filter((link) => link.hasAttribute('aria-current'))
    if (navLabel) {
      expect(primary.getByRole('link', { name: navLabel })).toHaveAttribute('aria-current', 'page')
      expect(current).toHaveLength(1)
    } else expect(current).toHaveLength(0)
  }
  expect(paidActions(view.procedures)).toEqual([])
})

it.each([
  ['/clips', '영상'],
  ['/settings', '설정'],
  ['/posts', '네이버 블로그'],
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
  const view = renderLegacyModelRoute('/posts', { user: { id: 'alice' } })
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
    // Guide: a retained record is a focused screen of AI 고르기's comparison flow — the tab bar
    // steps aside and the record's own top row carries the way back.
    expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
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

// The old Instagram list is one 영상 list now: its address lands there, still a root.
it.each([
  ['/posts/', '네이버 블로그', undefined],
  ['/settings/', '설정', undefined],
  ['/clips/', '영상', '/clips'],
  ['/naver-blog/', '네이버 블로그', '/posts'],
  ['/naver-clip/', '영상', '/clips'],
  ['/insta-clip/', '영상', '/clips'],
])('keeps trailing-slash root %s independent of explicit entry', async (path, label, landing) => {
  const view = renderLegacyModelRoute(`${path}?entry=%2Fposts`, { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: label, level: 1 })
  if (landing) expect(view.router.state.location.pathname).toBe(landing)
  expectRoot()
  expect(paidActions(view.procedures)).toEqual([])
})

it('retains the Instagram scope through shared templates, child navigation, reload and return', async () => {
  const calls: string[] = []
  const view = renderAppAt('/video-templates', {
    user: { id: 'alice' },
    calls,
  })
  await screen.findByRole('heading', { name: '영상 템플릿', level: 1 })
  // Guide: 템플릿's 글 · 영상 switch keeps the channel the page was opened under.
  expect(
    within(screen.getByRole('navigation', { name: '글 · 영상' })).getByRole('link', {
      name: '영상',
    }),
  ).toHaveAttribute('aria-current', 'page')
  expect(backHref()).toBeUndefined()
  await userEvent.click(await screen.findByRole('link', { name: '새 템플릿' }))
  await screen.findByRole('heading', { name: '새 영상 템플릿', level: 1 })
  // The wizard's address carries no channel, and its way back is the video template list.
  const address = view.router.state.location.href
  expect(address).toBe('/video-templates/new')
  view.unmount()
  sessionStorage.clear()
  const reopened = renderAppAt(address, { transport: view.transport })
  await screen.findByRole('heading', { name: '새 영상 템플릿', level: 1 })
  await userEvent.click(await frameReturn('영상 템플릿'))
  // Back on a section list page: the section row marks it and there is no way back further.
  await waitFor(() => expect(reopened.router.state.location.pathname).toBe('/video-templates'))
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
          videoTemplateId: '',
          ratio: 'vertical',
          targetDurationMs: 15000,
          disclosure: 'ad',
        },
      ],
    },
  })
  await screen.findByRole('link', { name: '영상으로 돌아가기' })
  await act(() => view.router.navigate({ to: '/video-guidelines' }))
  await screen.findByRole('heading', { name: '영상 지침', level: 1 })
  expect(backHref()).toBeUndefined()
  expect(paidActions(calls)).toEqual([])
})

it.each([
  ['템플릿', '/templates', '영상 템플릿'],
  ['지침', '/guidelines', '영상 지침'],
])('opens the %s 영상 side with no channel in its link', async (section, from, heading) => {
  const calls: string[] = []
  const view = renderAppAt(`${from}`, { user: { id: 'alice' }, calls })
  const kinds = await screen.findByRole('navigation', { name: '글 · 영상' })
  const href = within(kinds).getByRole('link', { name: '영상' }).getAttribute('href')!
  expect(href).not.toContain('channel=')
  view.unmount()
  sessionStorage.clear()
  renderAppAt(href, { transport: view.transport })
  await screen.findByRole('heading', { name: heading, level: 1 })
  expect(backHref()).toBeUndefined()
  const nav = screen.getByRole('navigation', { name: '주요' })
  expect(within(nav).getByRole('link', { name: '홈' })).toHaveAttribute('aria-current', 'page')
  // The section's creation button carries no channel (THEME-67).
  expect(
    await screen.findByRole('link', { name: section === '템플릿' ? '새 템플릿' : '새 지침' }),
  ).not.toHaveAttribute('href', expect.stringContaining('channel='))
  expect(paidActions(calls)).toEqual([])
})

// Guide: the kit's five sections are siblings in one chip row, whatever channel a page names,
// and they light 홈, which draws the kit.
const STYLE_HREFS = ['/voices', '/templates', '/guidelines', '/memories', '/spoken-voices']
it.each([
  ['/templates', '템플릿'],
  ['/voices', '말투'],
  ['/memories', '기억'],
] as const)(
  'places the %s section in the kit as a sibling with no way back',
  async (path, section) => {
    const view = renderAppAt(path, { user: { id: 'alice' } })
    const sections = await screen.findByRole('navigation', { name: '나누어 보기' })
    expect(
      within(sections)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual(STYLE_HREFS)
    expect(within(sections).getByRole('link', { name: section })).toHaveAttribute(
      'aria-current',
      'page',
    )
    expect(
      within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', { name: '홈' }),
    ).toHaveAttribute('aria-current', 'page')
    expect(backHref()).toBeUndefined()
    await userEvent.click(within(sections).getAllByRole('link')[0])
    await waitFor(() => expect(view.router.state.location.pathname).toBe(STYLE_HREFS[0]))
    expect(readNavigationEntry('alice', STYLE_HREFS[0])).toBeUndefined()
  },
)

it('returns a voice page opened from an Instagram 말투 list to that list', async () => {
  const view = renderAppAt('/voices', { user: { id: 'alice' } })
  await screen.findByRole('navigation', { name: '나누어 보기' })
  await act(() =>
    view.router.navigate({ to: '/voices/$voiceId', params: { voiceId: 'voice-default' } }),
  )
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/voices/voice-default'))
  // The voice screen's frame draws the one way back, beside the voice's name.
  expect(await frameReturn('말투')).toHaveAttribute('href', '/voices')
  expect(screen.queryByRole('navigation', { name: '나누어 보기' })).toBeNull()
  expect(
    within(screen.getByRole('navigation', { name: '주요' })).getByRole('link', {
      name: '홈',
    }),
  ).toHaveAttribute('aria-current', 'page')
})
