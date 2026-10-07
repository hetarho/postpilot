import { cleanup, screen, within } from '@testing-library/react'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'

beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  localStorage.clear()
})

it('retains every named settings destination when groups become a compact phone grid', async () => {
  renderAppAt('/settings', { user: { id: 'mobile-settings' } })
  const heading = await screen.findByRole('heading', { name: '설정', level: 1 })
  const main = heading.closest('main')!
  const links = within(main).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
    '/voices',
    '/templates',
    '/guidelines',
    '/memories',
    '/video-templates',
    '/video-guidelines',
    '/spoken-voices',
    '/ai-models',
    '/account',
    '/plans',
    '/billing',
    '/setup?restart=true',
    '/admin',
  ])
  for (const link of links) expect(link).toHaveAccessibleName()
  expect(within(main).getAllByRole('navigation')).toHaveLength(3)
})
