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

function breadcrumb() {
  return within(screen.getByRole('navigation', { name: '현재 위치' }))
}
function breadcrumbHrefs() {
  return breadcrumb()
    .queryAllByRole('link')
    .map((link) => link.getAttribute('href'))
}
function expectRoot(label: string) {
  expect(screen.getByRole('navigation', { name: '현재 위치' })).toHaveTextContent(label)
  expect(breadcrumbHrefs()).toEqual([])
  expect(screen.queryByRole('link', { name: '돌아가기' })).not.toBeInTheDocument()
  const sectionLinks = within(screen.getByRole('main'))
    .queryAllByRole('link')
    .map((link) => link.getAttribute('href'))
    .filter((href) =>
      ['/', '/tests', '/settings', '/library', '/posts', '/clips'].includes(href ?? ''),
    )
  expect(sectionLinks).toEqual(label === '작업 내역' ? ['/posts', '/clips'] : [])
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

it('traverses history, writing tests and settings as sibling destinations without paid work', async () => {
  const user = userEvent.setup()
  const view = renderLegacyModelRoute('/library', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '작업 내역', level: 1 })
  expectRoot('작업 내역')
  for (const [label, pathname] of [
    ['글쓰기 테스트', '/tests'],
    ['설정', '/settings'],
    ['작업 내역', '/library'],
  ] as const) {
    const primary = within(screen.getByRole('navigation', { name: '주요' }))
    await user.click(primary.getByRole('link', { name: label }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe(pathname))
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot(label)
    expect(primary.getByRole('link', { name: label })).toHaveAttribute('aria-current', 'page')
  }
  expect(paidActions(view.procedures)).toEqual([])
})

it.each([
  ['/tests', '글쓰기 테스트'],
  ['/settings', '설정'],
  ['/library', '작업 내역'],
])(
  'ignores an old cross-destination origin on direct load and reload of %s',
  async (path, label) => {
    legacyEntry(path, '/posts')
    const view = renderLegacyModelRoute(path, { user: { id: 'alice' } })
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot(label)
    view.unmount()
    renderAppAt(path, { transport: view.transport })
    await screen.findByRole('heading', { name: label, level: 1 })
    expectRoot(label)
    expect(paidActions(view.procedures)).toEqual([])
  },
)

it('does not make a generic entry parameter the parent of top-level writing tests', async () => {
  const view = renderLegacyModelRoute('/tests?entry=%2Flibrary', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '글쓰기 테스트', level: 1 })
  expectRoot('글쓰기 테스트')
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
  expect(breadcrumbHrefs()).toEqual(['/tests'])
  const back = within(screen.getByRole('main')).getByRole('link', { name: '글쓰기 테스트' })
  expect(back).toHaveAttribute('href', '/tests')
  await userEvent.click(back)
  await waitFor(() => expect(view.router.state.location.pathname).toBe('/tests'))
  expectRoot('글쓰기 테스트')
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
  expect(breadcrumbHrefs()).toEqual(['/tests'])
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
    expect(breadcrumbHrefs()).toEqual(['/tests'])
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
  expect(breadcrumbHrefs()).toEqual([])
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
  expect(breadcrumbHrefs()).toEqual([])
  expect(within(screen.getByRole('main')).queryByRole('link', { name: '글 작성' })).toBeNull()
  expect(screen.queryByRole('link', { name: '돌아가기' })).toBeNull()
  expect(paidActions(view.procedures)).toEqual([])
})

it('does not propagate a sibling visit into a settings child origin', async () => {
  const view = renderLegacyModelRoute('/library', { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: '작업 내역', level: 1 })
  await act(() => view.router.navigate({ to: '/templates' }))
  await screen.findByRole('heading', { name: '템플릿', level: 1 })
  expect(breadcrumbHrefs()).toEqual(['/settings', '/settings#settings-writing'])
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
    expect(breadcrumbHrefs()).toEqual(['/tests', '/tests/history'])
    const primary = within(screen.getByRole('navigation', { name: '주요' }))
    expect(primary.getByRole('link', { name: '글쓰기 테스트' })).toHaveAttribute(
      'aria-current',
      'page',
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
  ['/tests/', '글쓰기 테스트'],
  ['/settings/', '설정'],
  ['/library/', '작업 내역'],
])('keeps trailing-slash root %s independent of explicit entry', async (path, label) => {
  const view = renderLegacyModelRoute(`${path}?entry=%2Fposts`, { user: { id: 'alice' } })
  await screen.findByRole('heading', { name: label, level: 1 })
  expectRoot(label)
  expect(paidActions(view.procedures)).toEqual([])
})
