import { cleanup, screen, waitFor, within } from '@testing-library/react'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  localStorage.clear()
})
// Guide: 나 opens on the credits left with one 충전, then grouped rows that each drill into one
// topic. What the owner taught the AI lives in 내 스타일, model choice in AI 고르기.
it('leads with the credits and one 충전, then groups plan and account rows', async () => {
  renderAppAt('/settings', { user: { id: 'mobile-settings', plan: ProtoPlan.MASTER } })
  const heading = await screen.findByRole('heading', { name: '나', level: 1 })
  const main = heading.closest('main')!
  const credits = within(main).getByRole('region', { name: '남은 크레딧' })
  // Master's 충전 reaches the packs' own disabled state, as a paid tier's does (QUOTA-75).
  expect(within(credits).getByRole('link', { name: '충전하기' })).toHaveAttribute(
    'href',
    '/billing#billing-charge',
  )
  const links = within(main).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
    '/billing#billing-charge',
    '/plans',
    '/billing',
    '/account',
    '/setup?restart=true',
    '/admin',
  ])
  for (const link of links) expect(link).toHaveAccessibleName()
  expect(
    within(within(main).getByRole('navigation', { name: '이용' }))
      .getAllByRole('link')
      .map((link) => link.getAttribute('href')),
  ).toEqual(['/plans', '/billing'])
  // Nothing about writing is here any more.
  for (const gone of ['/memories', '/spoken-voices', '/ai-models', '/tests'])
    expect(main.querySelector(`a[href="${gone}"]`)).toBeNull()
  // The display preferences are rows on the page itself.
  expect(within(main).getByRole('group', { name: '인터페이스 환경설정' })).toBeInTheDocument()
})

// QUOTA-75: a paid tier's 충전 goes to the billing screen's packs; a free account cannot buy a
// pack (BILL-9), so its 충전 goes to /plans to subscribe.
it.each([
  [ProtoPlan.BASIC, '/billing#billing-charge'],
  [ProtoPlan.FREE, '/plans'],
])('sends 충전 by tier: plan %s goes to %s', async (plan, href) => {
  renderAppAt('/settings', { user: { id: 'charge-by-tier', plan }, plans: { plan } })
  const heading = await screen.findByRole('heading', { name: '나', level: 1 })
  const credits = within(heading.closest('main')!).getByRole('region', { name: '남은 크레딧' })
  await waitFor(() =>
    expect(within(credits).getByRole('link', { name: '충전하기' })).toHaveAttribute('href', href),
  )
})
