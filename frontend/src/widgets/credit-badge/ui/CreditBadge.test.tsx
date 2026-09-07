import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice' }

describe('CreditBadge', () => {
  // QUOTA-27: the balance used to be two taps deep and `/plans` had no other entry, so the
  // figure itself is the link.
  it('carries the balance in the header and leads to the ladder', async () => {
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.BASIC },
      plans: {
        plan: ProtoPlan.BASIC,
        balance: { credits: 137, unlimited: false, monthlyGrant: 220 },
      },
    })

    const link = await screen.findByRole('link', { name: '플랜 Basic, 남은 크레딧 137' })
    expect(link).toHaveAttribute('href', '/plans')
    expect(link).toHaveTextContent('137 크레딧')
    // The target passes 44px in BOTH dimensions: the height floor plus horizontal padding,
    // since three digits of label is nowhere near 44px wide (THEME-23).
    expect(link).toHaveClass('min-h-11', 'px-2')

    // The tier name is the first thing to give way at 320px, so it is hidden below `sm:`
    // and the accessible name carries it instead.
    const tier = within(link).getByText('Basic')
    expect(tier).toHaveClass('hidden', 'sm:inline')
    expect(tier).toHaveAttribute('aria-hidden', 'true')
  })

  it('states unlimited for the operator tier and still links to the ladder', async () => {
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.MASTER },
      plans: { plan: ProtoPlan.MASTER, balance: { unlimited: true } },
    })

    const link = await screen.findByRole('link', { name: '플랜 운영자, 크레딧 제한 없음' })
    expect(link).toHaveAttribute('href', '/plans')
    expect(link).toHaveTextContent('제한 없음')
  })

  // A failed read leaves the way to `/plans` open and says nothing: the popover is where a
  // balance failure is reported, and a header that swapped a figure for an error would move
  // the whole cluster under the thumb.
  it('holds its box with a dash when the balance cannot be read', async () => {
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, planFails: true },
    })

    const link = await screen.findByRole('link', { name: /플랜/ })
    expect(link).toHaveTextContent('—')
    expect(link).toHaveAttribute('href', '/plans')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  // The header and the popover are two readers of one cache entry (QUOTA-26): opening the
  // panel over a figure that is already on screen must not ask the server again.
  it('shares one GetMyPlan read with the account popover', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.FREE },
      calls,
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    await screen.findByRole('link', { name: '플랜 Free, 남은 크레딧 50' })
    expect(calls.filter((call) => call === 'GetMyPlan')).toHaveLength(1)

    await user.click(await screen.findByRole('button', { name: '내 계정' }))
    const panel = await screen.findByRole('dialog', { name: '내 계정' })
    expect(await within(panel).findByText('50 크레딧')).toBeInTheDocument()
    expect(calls.filter((call) => call === 'GetMyPlan')).toHaveLength(1)
  })
})
