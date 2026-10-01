import { afterEach, describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakePlansOptions } from '@/test/plans'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }

const renderTab = (plans: FakePlansOptions = {}) =>
  renderAppAt('/admin/costs', { user: MASTER, plans: { plan: ProtoPlan.MASTER, ...plans } })

const rateSection = async (name = '적용 환율') =>
  within(await screen.findByRole('region', { name }))

describe('AdminCostsPage', () => {
  afterEach(() => initializeI18n('ko'))

  // QUOTA-25, QUOTA-65: the rate customers are priced at is read here and nowhere else.
  it('is the fifth admin tab, reachable from the keyboard', async () => {
    const user = userEvent.setup()
    renderTab()
    const tabs = within(await screen.findByRole('navigation', { name: '운영 관리' }))
    const links = tabs.getAllByRole('link')
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/admin',
      '/admin/models',
      '/admin/estimator',
      '/admin/vouchers',
      '/admin/costs',
    ])
    const tab = tabs.getByRole('link', { name: /비용·환율/ })
    for (let presses = 0; presses < 40 && document.activeElement !== tab; presses += 1) {
      await user.tab()
    }
    expect(tab).toHaveFocus()
  })

  it('shows the confirmed rate in effect with its source, date and both values', async () => {
    renderTab()
    const section = await rateSection()
    expect(await section.findByText('korea-eximbank')).toBeInTheDocument()
    expect(section.getByText('2026-09-30')).toBeInTheDocument()
    expect(section.getByText('1,358.4원/USD')).toBeInTheDocument()
    expect(section.getByText('1,360원/USD')).toBeInTheDocument()
    expect(section.queryByText('임시 적용')).not.toBeInTheDocument()
  })

  // QUOTA-59: the last confirmed publication may stand in for seven days, flagged here in text.
  it('labels a temporary fallback in text', async () => {
    renderTab({
      exchangeRate: {
        source: 'korea-eximbank',
        publicationDate: '2026-09-26',
        referenceE4: 13_584_000n,
        appliedE4: 13_600_000n,
        temporary: true,
      },
    })
    const section = await rateSection()
    expect(await section.findByText('임시 적용')).toBeInTheDocument()
    expect(section.getByText(/7일 안에 확인된 최근 환율/)).toBeInTheDocument()
  })

  it('says what a missing rate stops instead of showing figures', async () => {
    renderTab({ exchangeRate: 'unavailable' })
    const section = await rateSection()
    expect(await section.findByText(/유료 AI 작업을 시작할 수 없어요/)).toBeInTheDocument()
    expect(section.queryByText(/원\/USD/)).not.toBeInTheDocument()
  })

  it('offers a retry after a failed read', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin/costs', {
      user: MASTER,
      calls,
      plans: { plan: ProtoPlan.MASTER, exchangeRateFailures: 1 },
    })
    const section = await rateSection()
    expect(await section.findByText('환율을 불러오지 못했어요.')).toBeInTheDocument()
    await user.click(section.getByRole('button', { name: '다시 시도' }))
    expect(await section.findByText('1,360원/USD')).toBeInTheDocument()
  })

  it('reads in English', async () => {
    initializeI18n('en')
    renderTab()
    const section = await rateSection('Exchange rate in effect')
    expect(await section.findByText('₩1,360/USD')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Costs & rate/ })).toHaveAttribute(
      'href',
      '/admin/costs',
    )
  })

  it('sends a non-operator back to the app', async () => {
    renderAppAt('/admin/costs', {
      user: { id: 'alice', plan: ProtoPlan.MAX },
      plans: { plan: ProtoPlan.MAX },
    })
    expect(await screen.findByRole('heading', { name: '내 글' })).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: '적용 환율' })).not.toBeInTheDocument()
  })
})
