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
  ['/clips/new', '/clips'],
  ['/clips/one', '/clips'],
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
/** The group level is drawn twice — the band that stands in for the rail below the desk, and the
 *  rail itself — because the two sit in different places in the document (THEME-38). */
function assertGroup(label: string, tab: string) {
  const shapes = screen.getAllByRole('navigation', { name: label })
  expect(shapes).toHaveLength(2)
  for (const shape of shapes) {
    const links = within(shape).getAllByRole('link')
    expect(
      links
        .filter((l) => l.getAttribute('aria-current') === 'page')
        .map((l) => l.getAttribute('href')),
    ).toEqual([tab])
  }
  return shapes
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
    expect(band).toHaveClass('overflow-x-auto', 'overscroll-x-contain')
    expect(band!.className).not.toMatch(/(?:fixed|overflow-y-auto)/)
    expect(rail!.closest('aside')).toHaveClass('lg:sticky', 'lg:top-header', 'lg:h-sidebar')
    // Everything under the group clears the taller chrome; the page stays the one scroller.
    expect(band!.closest('.chrome-subnav')).not.toBeNull()
    expect(band!.closest('.pb-nav')?.querySelectorAll('[class~="overflow-y-auto"]')).toHaveLength(0)
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
    within(screen.getAllByRole('navigation', { name: '글 메뉴' })[0]!).getByRole('link', {
      name: '말투',
    }),
  )
  await waitFor(() => expect(router.state.location.pathname).toBe('/voices'))
  await userEvent.click(
    within(screen.getAllByRole('navigation', { name: '주요' })[0]!).getByRole('link', {
      name: '영상',
    }),
  )
  await screen.findAllByRole('navigation', { name: '영상 메뉴' })
  await userEvent.click(
    within(screen.getAllByRole('navigation', { name: '영상 메뉴' })[0]!).getByRole('link', {
      name: '영상 템플릿',
    }),
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
  expect(within(band!).getByRole('link', { name: 'Video templates' })).toHaveAttribute(
    'href',
    '/video-templates',
  )
})
