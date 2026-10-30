import { cleanup, screen, within } from '@testing-library/react'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  localStorage.clear()
})
it('groups personal and account settings with tests under advanced, leaving 말투, templates and guidelines in channels', async () => {
  renderAppAt('/settings', { user: { id: 'mobile-settings' } })
  const heading = await screen.findByRole('heading', { name: '내 설정', level: 1 })
  const main = heading.closest('main')!
  const links = within(main).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
    '/memories',
    '/spoken-voices',
    '/ai-models',
    '/account',
    '/plans',
    '/billing',
    '/tests',
    '/tests/history',
    '/setup?restart=true',
    '/admin',
  ])
  for (const link of links) expect(link).toHaveAccessibleName()
  expect(within(main).getAllByRole('navigation')).toHaveLength(4)
  const advanced = within(main).getByRole('navigation', { name: '고급' })
  expect(
    within(advanced)
      .getAllByRole('link')
      .map((link) => link.getAttribute('href')),
  ).toEqual(['/tests', '/tests/history'])
})
