import { afterEach, expect, it } from 'vitest'
import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { currentDestination } from './navigation'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

const writing = [
  ['/posts', '/posts'],
  ['/posts/new', '/posts'],
  ['/posts/example', '/posts'],
  ['/posts/experiments/one', '/posts'],
  ['/voices', '/voices'],
  ['/voices/one', '/voices'],
  ['/voices/one/versions', '/voices'],
  ['/voices/one/import', '/voices'],
  ['/voices/one/rules', '/voices'],
  ['/voices/one/validations', '/voices'],
  ['/voices/one/rules/two/compare', '/voices'],
  ['/voices/one/validations/two', '/voices'],
  ['/templates', '/templates'],
  ['/templates/new', '/templates'],
  ['/templates/one', '/templates'],
  ['/guidelines', '/guidelines'],
] as const
const video = [
  ['/clips', '/clips'],
  ['/video-templates', '/video-templates'],
  ['/video-templates/new', '/video-templates'],
  ['/video-templates/one', '/video-templates'],
] as const
const models = [
  ['/ai-models', '/ai-models'],
  ['/ai-models/compare', '/ai-models/compare'],
  ['/ai-models/experiments', '/ai-models/experiments'],
  ['/ai-models/leaderboard', '/ai-models/leaderboard'],
  ['/ai-models/experiments/one', '/ai-models/experiments'],
] as const
const cases = [
  ...models.map(([path, tab]) => ({ path, tab, primary: '/ai-models', group: 'AI 모델 메뉴' })),
  ...writing.map(([path, tab]) => ({ path, tab, primary: '/posts', group: '글 메뉴' })),
  ...video.map(([path, tab]) => ({ path, tab, primary: '/clips', group: '영상 메뉴' })),
]

/** The desk's one sidebar. It holds BOTH levels since 2026-09-22, so a row says which level it
 *  is: a group's home repeats its primary destination's address (내 글 IS /posts) and nothing
 *  could tell the two apart by href. */
const rail = () => document.querySelector('aside')!
const rows = (level: 'primary' | 'group', scope: HTMLElement = rail()) =>
  within(scope)
    .queryAllByRole('link')
    .filter((link) => link.dataset.navLevel === level)

/** The primary level is drawn three times: the laptop's band, the desk's rail, the phone's bar.
 *  Only the rail carries a second level, which is filtered out here. */
function assertPrimary(current: string | undefined, label = '주요') {
  const shapes = screen.getAllByRole('navigation', { name: label })
  expect(shapes).toHaveLength(3)
  for (const shape of shapes) {
    const links = within(shape)
      .getAllByRole('link')
      .filter((link) => link.dataset.navLevel !== 'group')
    expect(links.map((l) => l.getAttribute('href'))).toEqual(['/posts', '/clips', '/ai-models'])
    expect(
      links
        .filter((l) => l.getAttribute('aria-current') === 'page')
        .map((l) => l.getAttribute('href')),
    ).toEqual(current ? [current] : [])
  }
}

/** The group level is drawn twice, in the two places it fits: inside the desk's one sidebar,
 *  indented under the destination that opens it, and as ONE menu control in the middle of the
 *  brand row at every narrower width (owner decision 2026-09-22). */
function assertGroup(label: string, tab: string) {
  const band = screen.getByRole('navigation', { name: label })
  const links = rows('group')
  expect(
    links
      .filter((l) => l.getAttribute('aria-current') === 'page')
      .map((l) => l.getAttribute('href')),
  ).toEqual([tab])
  const current = links.find((l) => l.getAttribute('href') === tab)!
  expect(within(band).queryAllByRole('link')).toHaveLength(0)
  // One control, and the name of the place IS that control — not a name beside a menu button at
  // the far right of the row. Its visible text is its accessible name (WCAG 2.5.3).
  const trigger = within(band).getByRole('button')
  expect(trigger).toHaveAccessibleName(current.textContent!)
  expect(trigger).toHaveAttribute('aria-haspopup', 'menu')
  return [band, rail()] as const
}
/** Opens the brand row's group menu and answers with the open menu, scoped. */
async function openGroupMenu(label: string) {
  const band = screen.getByRole('navigation', { name: label })
  await userEvent.click(within(band).getByRole('button'))
  return within(await screen.findByRole('menu', { name: label }))
}
it.each(cases)(
  'preserves the direct $path address and both matched navigation levels',
  async ({ path, tab, primary, group }) => {
    const { router } = renderAppAt(path, { user: { id: 'alice', plan: ProtoPlan.FREE } })
    await screen.findAllByRole('navigation', { name: group })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(router.state.location.pathname).toBe(path)
    assertPrimary(primary)
    const [band, railEl] = assertGroup(group, tab)
    // The band is chrome that stays put while the page scrolls: `top-0` while the header still
    // scrolls away, under the header once that is sticky, and never its own vertical scroller.
    // The group's control rides the header, which is the one thing that sticks; the rail hangs
    // from the header's bottom edge. Neither is on the page, so the page stays the one scroller.
    expect(band.closest('header')).not.toBeNull()
    expect(band.className).not.toMatch(/(?:fixed|sticky|overflow-y-auto|overflow-x-auto)/)
    expect(railEl).toHaveClass('lg:sticky', 'lg:top-header', 'lg:h-sidebar')
    expect(
      document.querySelector('.pb-nav')?.querySelectorAll('[class~="overflow-y-auto"]'),
    ).toHaveLength(0)
  },
)

// The clip workspace is a page inside the group that asks for the group level to be absent
// (CLIP-37): the primary level still marks 영상, but no group row or rail is drawn and nothing
// under it clears a group row that is not there.
it.each(['/clips/new', '/clips/one'])(
  'draws no group level on the clip workspace at %s',
  async (path) => {
    const { router } = renderAppAt(path, { user: { id: 'alice', plan: ProtoPlan.FREE } })
    await screen.findAllByRole('navigation', { name: '주요' })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(router.state.location.pathname).toBe(path)
    assertPrimary('/clips')
    expect(screen.queryAllByRole('navigation', { name: '영상 메뉴' })).toHaveLength(0)
    expect(rows('group')).toHaveLength(0)
  },
)

it.each(['/plans', '/billing', '/account', '/admin', '/admin/models', '/admin/estimator'])(
  'keeps primary destinations visible but unselected on %s',
  async (path) => {
    const { router } = renderAppAt(path, { user: { id: 'root', plan: ProtoPlan.MASTER } })
    await screen.findAllByRole('navigation', { name: '주요' })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    assertPrimary(undefined)
    expect(screen.queryAllByRole('navigation', { name: '글 메뉴' })).toHaveLength(0)
    expect(screen.queryAllByRole('navigation', { name: '영상 메뉴' })).toHaveLength(0)
    expect(rows('group')).toHaveLength(0)
    expect(screen.getByRole('link', { name: 'Postpilot 홈' })).toHaveAttribute('href', '/posts')
  },
)
it.each(['/ai-models', '/ai-models/experiments/one'])(
  'marks the standalone matched destination at %s',
  async (path) => {
    const { router } = renderAppAt(path, { user: { id: 'root', plan: ProtoPlan.MASTER } })
    await screen.findAllByRole('navigation', { name: '주요' })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    assertPrimary('/ai-models')
  },
)
it('uses actual matched ids instead of prefix guesses', () => {
  expect(currentDestination(['/authenticated/writing', '/authenticated/writing/voices'])).toBe(
    '/posts',
  )
  expect(currentDestination(['/authenticated/video', '/authenticated/video/clips/$clipId'])).toBe(
    '/clips',
  )
  expect(currentDestination(['/authenticated/writing-lookalike', '/postsish'])).toBeUndefined()
  expect(currentDestination(['/authenticated/admin/models'])).toBeUndefined()
  expect(
    currentDestination(['/authenticated/models', '/authenticated/models/ai-models/compare']),
  ).toBe('/ai-models')
})
it('restores both active levels through browser history and keeps ko/en parity', async () => {
  const { router } = renderAppAt('/posts', { user: { id: 'root', plan: ProtoPlan.MASTER } })
  await screen.findAllByRole('navigation', { name: '글 메뉴' })
  await userEvent.click(
    (await openGroupMenu('글 메뉴')).getByRole('menuitemradio', { name: '말투' }),
  )
  await waitFor(() => expect(router.state.location.pathname).toBe('/voices'))
  await userEvent.click(
    within(screen.getAllByRole('navigation', { name: '주요' })[0]!).getByRole('link', {
      name: '영상',
    }),
  )
  await screen.findAllByRole('navigation', { name: '영상 메뉴' })
  await userEvent.click(
    (await openGroupMenu('영상 메뉴')).getByRole('menuitemradio', { name: '영상 템플릿' }),
  )
  await waitFor(() => expect(router.state.location.pathname).toBe('/video-templates'))
  assertPrimary('/clips')
  await act(async () => {
    router.history.back()
  })
  await waitFor(() => expect(router.state.location.pathname).toBe('/clips'))
  assertPrimary('/clips')
  await act(async () => {
    router.history.back()
  })
  await screen.findAllByRole('navigation', { name: '글 메뉴' })
  expect(router.state.location.pathname).toBe('/voices')
  assertPrimary('/posts')
  assertGroup('글 메뉴', '/voices')
  await act(async () => {
    router.history.forward()
  })
  await screen.findAllByRole('navigation', { name: '영상 메뉴' })
  assertPrimary('/clips')
  await act(async () => {
    initializeI18n('en')
  })
  assertPrimary('/clips', 'Primary')
  expect(
    within(screen.getAllByRole('navigation', { name: 'Primary' })[0]!).getByRole('link', {
      name: 'Posts',
    }),
  ).toHaveClass('min-w-11', 'min-h-11')
  expect(
    within(screen.getAllByRole('navigation', { name: 'Primary' })[0]!)
      .getAllByRole('link')
      .map((l) => l.textContent),
  ).toEqual(['Posts', 'Videos', 'AI models'])
  const [band] = assertGroup('Video navigation', '/clips')
  expect(band).toHaveTextContent('My videos')
  // The one sidebar lists both levels in one column, the group's under the row that opened it.
  expect(
    within(rail())
      .getAllByRole('link')
      .map((l) => `${l.dataset.navLevel}:${l.getAttribute('href')}`),
  ).toEqual([
    'primary:/posts',
    'primary:/clips',
    'group:/clips',
    'group:/video-templates',
    'primary:/ai-models',
  ])
  const menu = await openGroupMenu('Video navigation')
  expect(menu.getAllByRole('menuitemradio').map((item) => item.textContent)).toEqual([
    'My videos',
    'Video templates',
  ])
  expect(menu.getByRole('menuitemradio', { name: 'My videos' })).toHaveAttribute(
    'aria-checked',
    'true',
  )
})

// THEME-38 (owner decision 2026-09-19): below the desk the group level is the current
// destination's name — the group's home by default — with one menu control holding the group, in
// place of a row of pill links that read as buttons rather than as a menu.
it.each([
  ['/posts', '글 메뉴', ['내 글', '말투', '글 템플릿', '지침', '기억'], '/voices'],
  ['/clips', '영상 메뉴', ['내 영상', '영상 템플릿'], '/video-templates'],
  [
    '/ai-models',
    'AI 모델 메뉴',
    ['모델 변경', '모델 비교', '최근 관찰 비교', '리더보드'],
    '/ai-models/compare',
  ],
] as const)(
  'names the group home at %s and keeps the rest of the group behind one menu',
  async (path, label, all, second) => {
    const { router } = renderAppAt(path, { user: { id: 'alice', plan: ProtoPlan.FREE } })
    const [band] = await screen.findAllByRole('navigation', { name: label })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(band).toHaveTextContent(all[0])
    expect(within(band!).queryAllByRole('link')).toHaveLength(0)
    const menu = await openGroupMenu(label)
    const items = menu.getAllByRole('menuitemradio')
    expect(items.map((item) => item.textContent)).toEqual([...all])
    expect(items[0]).toHaveAttribute('aria-checked', 'true')
    await userEvent.click(items[1]!)
    await waitFor(() => expect(router.state.location.pathname).toBe(second))
    expect(screen.queryByRole('menu')).not.toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: label })).toHaveTextContent(all[1])
  },
)

// The desk rail FOLDS rather than leaves (owner decision 2026-09-22): the destinations stay, the
// names go, and the way back moves to the brand row, where a folded column has no room for it.
it('folds the one sidebar down to its glyphs and back', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findAllByRole('navigation', { name: '주요' })
  await waitFor(() => expect(router.state.status).toBe('idle'))
  expect(screen.queryByRole('button', { name: '주요 메뉴 열기' })).not.toBeInTheDocument()

  // One control for both halves, beside the brand mark at either width.
  const close = screen.getByRole('button', { name: '주요 메뉴 닫기' })
  expect(close.closest('header')).not.toBeNull()
  expect(close).toHaveAttribute('aria-expanded', 'true')
  const named = within(rail()).getAllByRole('link')
  expect(named[0]).toHaveTextContent(/\S/)

  await user.click(close)

  // Folded: the same rail, both levels intact, every name still on the link for a screen reader.
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '주요 메뉴 열기' })).toBeInTheDocument(),
  )
  expect(screen.queryByRole('button', { name: '주요 메뉴 닫기' })).not.toBeInTheDocument()
  // The same control, in the same place, now saying the other half.
  const open = screen.getByRole('button', { name: '주요 메뉴 열기' })
  expect(open).toBe(close)
  expect(open).toHaveAttribute('aria-expanded', 'false')
  expect(within(rail()).queryAllByRole('button')).toHaveLength(0)
  const glyphs = within(rail()).getAllByRole('link')
  expect(glyphs.map((l) => l.getAttribute('href'))).toEqual(
    named.map((l) => l.getAttribute('href')),
  )
  expect(rows('group').length).toBeGreaterThan(0)
  for (const link of glyphs) {
    expect(link).toHaveClass('size-11')
    expect(link).toHaveAccessibleName()
    // The name is not gone, it is a tooltip the app draws beside the glyph on hover or focus.
    const tip = link.querySelector('span')!
    expect(tip).toHaveTextContent(/\S/)
    expect(tip).toHaveClass('hidden', 'group-hover:block', 'group-focus-visible:block')
  }
  expect(router.state.location.pathname).toBe('/posts')

  await user.click(screen.getByRole('button', { name: '주요 메뉴 열기' }))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: '주요 메뉴 닫기' })).toBeInTheDocument(),
  )
  expect(within(rail()).getAllByRole('link')[0]).toHaveTextContent(/\S/)
  assertPrimary('/posts')
})

// One sidebar, two levels: pressing a primary destination opens ITS group under it, and the
// group that was open closes with the address that held it.
it('moves the open group in the sidebar when the primary destination changes', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findAllByRole('navigation', { name: '글 메뉴' })
  await waitFor(() => expect(router.state.status).toBe('idle'))
  expect(rows('group').map((l) => l.getAttribute('href'))).toEqual([
    '/posts',
    '/voices',
    '/templates',
    '/guidelines',
    '/memories',
  ])

  await user.click(within(rail()).getByRole('link', { name: '영상' }))
  await waitFor(() => expect(router.state.location.pathname).toBe('/clips'))

  expect(rows('group').map((l) => l.getAttribute('href'))).toEqual(['/clips', '/video-templates'])
  assertPrimary('/clips')
  assertGroup('영상 메뉴', '/clips')
  // A group destination of the group that closed is no longer anywhere in the sidebar.
  expect(within(rail()).queryByRole('link', { name: '말투' })).toBeNull()
})
