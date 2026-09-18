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
const cases = [
  ...writing.map(([path, tab]) => ({ path, tab, primary: '/posts', group: '글 메뉴' })),
  ...video.map(([path, tab]) => ({ path, tab, primary: '/clips', group: '영상 메뉴' })),
]

function assertPrimary(current: string | undefined, master = false, label = '주요') {
  const shapes = screen.getAllByRole('navigation', { name: label })
  expect(shapes).toHaveLength(3)
  for (const shape of shapes) {
    const links = within(shape).getAllByRole('link')
    expect(links.map((l) => l.getAttribute('href'))).toEqual([
      '/posts',
      '/clips',
      '/ai-models',
      ...(master ? ['/publishing-agents'] : []),
    ])
    expect(
      links
        .filter((l) => l.getAttribute('aria-current') === 'page')
        .map((l) => l.getAttribute('href')),
    ).toEqual(current ? [current] : [])
  }
}
/** The group level is drawn twice — the band below the desk, which names the current destination
 *  and holds the rest behind one menu control, and the rail on the desk, which lists every
 *  destination as a link — because the two sit in different places in the document (THEME-38). */
function assertGroup(label: string, tab: string) {
  const shapes = screen.getAllByRole('navigation', { name: label })
  expect(shapes).toHaveLength(2)
  const [band, rail] = shapes
  const links = within(rail!).getAllByRole('link')
  expect(
    links
      .filter((l) => l.getAttribute('aria-current') === 'page')
      .map((l) => l.getAttribute('href')),
  ).toEqual([tab])
  const current = links.find((l) => l.getAttribute('href') === tab)!
  expect(within(band!).queryAllByRole('link')).toHaveLength(0)
  expect(band).toHaveTextContent(current.textContent!)
  expect(within(band!).getByRole('button', { name: label })).toHaveAttribute(
    'aria-haspopup',
    'menu',
  )
  return shapes
}
/** Opens the band's menu and answers with the open menu, scoped. */
async function openGroupMenu(label: string) {
  const [band] = screen.getAllByRole('navigation', { name: label })
  await userEvent.click(within(band!).getByRole('button', { name: label }))
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
    const [band, rail] = assertGroup(group, tab)
    // The band is chrome that stays put while the page scrolls: `top-0` while the header still
    // scrolls away, under the header once that is sticky, and never its own vertical scroller.
    expect(band).toHaveClass('sticky', 'top-0', 'sm:top-header', 'h-subnav')
    expect(band!.className).not.toMatch(/(?:fixed|overflow-y-auto|overflow-x-auto)/)
    expect(rail!.closest('aside')).toHaveClass('lg:sticky', 'lg:top-header', 'lg:h-sidebar')
    // Everything under the group clears the taller chrome; the page stays the one scroller.
    expect(band!.closest('.chrome-subnav')).not.toBeNull()
    expect(band!.closest('.pb-nav')?.querySelectorAll('[class~="overflow-y-auto"]')).toHaveLength(0)
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
    expect(document.querySelector('.chrome-subnav')).toBeNull()
  },
)

it.each(['/plans', '/billing', '/account', '/admin', '/admin/models', '/admin/estimator'])(
  'keeps primary destinations visible but unselected on %s',
  async (path) => {
    const { router } = renderAppAt(path, { user: { id: 'root', plan: ProtoPlan.MASTER } })
    await screen.findAllByRole('navigation', { name: '주요' })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    assertPrimary(undefined, true)
    expect(screen.queryAllByRole('navigation', { name: '글 메뉴' })).toHaveLength(0)
    expect(screen.queryAllByRole('navigation', { name: '영상 메뉴' })).toHaveLength(0)
    expect(document.querySelector('.chrome-subnav')).toBeNull()
    expect(screen.getByRole('link', { name: 'Postpilot 홈' })).toHaveAttribute('href', '/posts')
  },
)
it.each(['/ai-models', '/ai-models/experiments/one', '/publishing-agents'])(
  'marks the standalone matched destination at %s',
  async (path) => {
    const { router } = renderAppAt(path, { user: { id: 'root', plan: ProtoPlan.MASTER } })
    await screen.findAllByRole('navigation', { name: '주요' })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    assertPrimary(path === '/publishing-agents' ? path : '/ai-models', true)
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
  assertPrimary('/clips', true)
  await act(async () => {
    router.history.back()
  })
  await waitFor(() => expect(router.state.location.pathname).toBe('/clips'))
  assertPrimary('/clips', true)
  await act(async () => {
    router.history.back()
  })
  await screen.findAllByRole('navigation', { name: '글 메뉴' })
  expect(router.state.location.pathname).toBe('/voices')
  assertPrimary('/posts', true)
  assertGroup('글 메뉴', '/voices')
  await act(async () => {
    router.history.forward()
  })
  await screen.findAllByRole('navigation', { name: '영상 메뉴' })
  assertPrimary('/clips', true)
  await act(async () => {
    initializeI18n('en')
  })
  assertPrimary('/clips', true, 'Primary')
  const phone = screen.getAllByRole('navigation', { name: 'Primary' })[2]!
  const publish = within(phone).getByRole('link', { name: 'Publishing tools' })
  expect(publish).toHaveTextContent('Publish')
  expect(publish).toHaveClass('min-h-14', 'flex-1')
  expect(
    within(screen.getAllByRole('navigation', { name: 'Primary' })[0]!).getByRole('link', {
      name: 'Posts',
    }),
  ).toHaveClass('min-w-11', 'min-h-11')
  expect(
    within(screen.getAllByRole('navigation', { name: 'Primary' })[0]!)
      .getAllByRole('link')
      .map((l) => l.textContent),
  ).toEqual(['Posts', 'Videos', 'AI models', 'Publishing tools'])
  const [band] = assertGroup('Video navigation', '/clips')
  expect(band).toHaveTextContent('My videos')
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
  ['/posts', '글 메뉴', ['내 글', '말투', '글 템플릿', '지침'], '/voices'],
  ['/clips', '영상 메뉴', ['내 영상', '영상 템플릿'], '/video-templates'],
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
    expect(screen.getAllByRole('navigation', { name: label })[0]).toHaveTextContent(all[1])
  },
)
