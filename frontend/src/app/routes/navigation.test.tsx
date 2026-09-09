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
it.each(cases)(
  'preserves the direct $path address and both matched navigation levels',
  async ({ path, tab, primary, group }) => {
    const { router } = renderAppAt(path, { user: { id: 'alice', plan: ProtoPlan.FREE } })
    const tabs = await screen.findByRole('navigation', { name: group })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    expect(router.state.location.pathname).toBe(path)
    assertPrimary(primary)
    const selected = within(tabs)
      .getAllByRole('link')
      .filter((l) => l.getAttribute('aria-current') === 'page')
    expect(selected.map((l) => l.getAttribute('href'))).toEqual([tab])
    expect(tabs).toHaveClass('overflow-x-auto', 'overscroll-x-contain')
    expect(tabs.className).not.toMatch(/(?:fixed|sticky|overflow-y)/)
    expect(tabs.closest('.pb-nav')?.querySelectorAll('[class~="overflow-y-auto"]')).toHaveLength(0)
  },
)

it.each(['/plans', '/billing', '/account', '/admin', '/admin/models', '/admin/estimator'])(
  'keeps primary destinations visible but unselected on %s',
  async (path) => {
    const { router } = renderAppAt(path, { user: { id: 'root', plan: ProtoPlan.MASTER } })
    await screen.findAllByRole('navigation', { name: '주요' })
    await waitFor(() => expect(router.state.status).toBe('idle'))
    assertPrimary(undefined, true)
    expect(screen.queryByRole('navigation', { name: '글 메뉴' })).not.toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: '영상 메뉴' })).not.toBeInTheDocument()
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
  await screen.findByRole('navigation', { name: '글 메뉴' })
  await userEvent.click(
    within(screen.getByRole('navigation', { name: '글 메뉴' })).getByRole('link', { name: '말투' }),
  )
  await waitFor(() => expect(router.state.location.pathname).toBe('/voices'))
  await userEvent.click(
    within(screen.getAllByRole('navigation', { name: '주요' })[0]!).getByRole('link', {
      name: '영상',
    }),
  )
  await screen.findByRole('navigation', { name: '영상 메뉴' })
  await userEvent.click(
    within(screen.getByRole('navigation', { name: '영상 메뉴' })).getByRole('link', {
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
  await screen.findByRole('navigation', { name: '글 메뉴' })
  expect(router.state.location.pathname).toBe('/voices')
  assertPrimary('/posts', true)
  expect(
    within(screen.getByRole('navigation', { name: '글 메뉴' })).getByRole('link', { name: '말투' }),
  ).toHaveAttribute('aria-current', 'page')
  await act(async () => {
    router.history.forward()
  })
  await screen.findByRole('navigation', { name: '영상 메뉴' })
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
  const tabs = screen.getByRole('navigation', { name: 'Video navigation' })
  expect(within(tabs).getByRole('link', { name: 'Clips' })).toHaveAttribute('aria-current', 'page')
  expect(within(tabs).getByRole('link', { name: 'Video templates' })).toHaveAttribute(
    'href',
    '/video-templates',
  )
})
