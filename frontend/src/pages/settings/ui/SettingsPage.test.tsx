import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
beforeEach(() => initializeI18n('ko'))
afterEach(() => {
  cleanup()
  localStorage.clear()
})
// 설정 (behind the avatar) opens on the shared credits card with one 충전 (QUOTA-75), then grouped
// rows that each drill into one topic. What the owner taught the AI lives in 홈's kit, model
// choice in AI 토너먼트.
it('leads with the credits and one 충전, then groups plan and account rows', async () => {
  renderAppAt('/settings', { user: { id: 'mobile-settings', plan: ProtoPlan.MASTER } })
  const heading = await screen.findByRole('heading', { name: '설정', level: 1 })
  const main = heading.closest('main')!
  const credits = within(main).getByRole('region', { name: '남은 크레딧' })
  // Master's 충전 reaches the packs' own disabled state in a sheet, as a paid tier's reaches the
  // packs (QUOTA-75 →QUOTA-68).
  await userEvent.setup().click(within(credits).getByRole('button', { name: '충전하기' }))
  const sheet = screen.getByRole('dialog', { name: '크레딧 구매' })
  expect(await within(sheet).findByRole('button', { name: '크레딧 구매' })).toBeDisabled()
  await userEvent.setup().keyboard('{Escape}')
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  const links = within(main).getAllByRole('link')
  expect(links.map((link) => link.getAttribute('href'))).toEqual([
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

// QUOTA-75: a paid tier's 충전 brings the packs up in a sheet over 설정 itself, where a pack is
// bought exactly as on the billing screen; a free account cannot buy a pack (BILL-9), so its 충전
// goes to /plans to subscribe.
it('buys a pack from 설정 without leaving it', async () => {
  const user = userEvent.setup()
  const purchaseRequests: Array<number | string> = []
  renderAppAt('/settings', {
    user: { id: 'charge-paid', plan: ProtoPlan.BASIC },
    plans: { plan: ProtoPlan.BASIC },
    billing: { paymentMethod: true, subscription: true, purchaseRequests },
  })
  const heading = await screen.findByRole('heading', { name: '설정', level: 1 })
  const credits = within(heading.closest('main')!).getByRole('region', { name: '남은 크레딧' })
  await user.click(await within(credits).findByRole('button', { name: '충전하기' }))
  const sheet = screen.getByRole('dialog', { name: '크레딧 구매' })
  const buy = await within(sheet).findByRole('button', { name: '크레딧 구매' })
  await waitFor(() => expect(buy).toBeEnabled())
  await user.click(buy)
  await user.click(
    within(screen.getByRole('dialog', { name: '크레딧을 구매할까요?' })).getByRole('button', {
      name: '크레딧 구매',
    }),
  )
  await waitFor(() => expect(purchaseRequests).toEqual(['pack-1000']))
  expect(await within(sheet).findByText('1000 크레딧을 구매했습니다.')).toBeInTheDocument()
  expect(screen.getByRole('heading', { name: '설정', level: 1 })).toBeInTheDocument()
})

it('sends a free account’s 충전 to /plans', async () => {
  renderAppAt('/settings', {
    user: { id: 'charge-free', plan: ProtoPlan.FREE },
    plans: { plan: ProtoPlan.FREE },
  })
  const heading = await screen.findByRole('heading', { name: '설정', level: 1 })
  const credits = within(heading.closest('main')!).getByRole('region', { name: '남은 크레딧' })
  await waitFor(() =>
    expect(within(credits).getByRole('link', { name: '충전하기' })).toHaveAttribute(
      'href',
      '/plans',
    ),
  )
})
