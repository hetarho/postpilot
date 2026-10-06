import { afterEach, expect, it } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { currentDestination } from './navigation'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

it.each([
  ['/', '/'],
  ['/posts/new', '/'],
  ['/clips/new', '/'],
  ['/posts/example', '/library'],
  ['/clips/example', '/library'],
  ['/voices/example/materials', '/settings'],
  ['/ai-models/compare', '/settings'],
  ['/billing', '/settings'],
])('resolves %s to the menu destination %s', (pathname, current) => {
  expect(currentDestination(pathname)).toBe(current)
})

it('keeps navigation out of the canvas and opens one accessible menu on request', async () => {
  const user = userEvent.setup()
  renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await screen.findByRole('heading', { name: '내 글' })
  expect(screen.queryByRole('navigation', { name: '주요' })).toBeNull()
  expect(document.querySelector('aside')).toBeNull()
  expect(document.querySelector('nav.fixed')).toBeNull()
  const trigger = screen.getByRole('button', { name: '주요 메뉴 열기' })
  await user.click(trigger)
  const menu = screen.getByRole('dialog', { name: '메뉴' })
  const links = within(menu).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual(['/', '/library', '/settings'])
  expect(within(menu).getByRole('link', { name: '보관함' })).toHaveAttribute('aria-current', 'page')
  expect(menu.contains(document.activeElement)).toBe(true)
  await user.keyboard('{Escape}')
  expect(screen.queryByRole('dialog', { name: '메뉴' })).toBeNull()
  expect(trigger).toHaveFocus()
})

it('closes the menu after navigation and makes every configuration accessible from settings', async () => {
  const user = userEvent.setup()
  const { router } = renderAppAt('/posts', { user: { id: 'alice', plan: ProtoPlan.FREE } })
  await user.click(await screen.findByRole('button', { name: '주요 메뉴 열기' }))
  await user.click(screen.getByRole('link', { name: '설정' }))
  await screen.findByRole('heading', { name: '설정' })
  expect(router.state.location.pathname).toBe('/settings')
  expect(screen.queryByRole('dialog', { name: '메뉴' })).toBeNull()
  const main = screen.getByRole('main')
  for (const path of [
    '/voices',
    '/templates',
    '/guidelines',
    '/memories',
    '/spoken-voices',
    '/video-templates',
    '/video-guidelines',
    '/ai-models',
    '/ai-models/compare',
    '/ai-models/experiments',
    '/ai-models/leaderboard',
    '/account',
    '/plans',
    '/billing',
  ])
    expect(
      within(main)
        .getAllByRole('link')
        .some((link) => link.getAttribute('href') === path),
    ).toBe(true)
  expect(within(main).queryByRole('link', { name: '관리자' })).toBeNull()
  await router.navigate({ to: '/library' })
  expect(await screen.findByRole('link', { name: /내 글/ })).toHaveAttribute('href', '/posts')
  expect(screen.getByRole('link', { name: /내 클립/ })).toHaveAttribute('href', '/clips')
  await waitFor(() => expect(router.state.status).toBe('idle'))
})

it('exposes administration only to the operator in settings', async () => {
  renderAppAt('/settings', { user: { id: 'operator', plan: ProtoPlan.MASTER } })
  expect(await screen.findByRole('link', { name: '관리자' })).toHaveAttribute('href', '/admin')
})

it.each([
  ['ko', '새 글 작성하기', '새 클립 만들기'],
  ['en', 'Write a new post', 'Create a new clip'],
] as const)(
  'shows exactly two creation choices in %s without object writes',
  async (locale, post, clip) => {
    initializeI18n(locale)
    const calls: string[] = []
    const { router } = renderAppAt('/', { user: { id: 'alice' }, calls })
    const main = await screen.findByRole('main')
    expect(within(main).getAllByRole('link')).toHaveLength(2)
    expect(within(main).getByRole('link', { name: new RegExp(post) })).toHaveAttribute(
      'href',
      '/posts/new',
    )
    expect(within(main).getByRole('link', { name: new RegExp(clip) })).toHaveAttribute(
      'href',
      '/clips/new',
    )
    expect(router.state.location.pathname).toBe('/')
    expect(calls.every((call) => !/Create|Start|Save/.test(call))).toBe(true)
  },
)
