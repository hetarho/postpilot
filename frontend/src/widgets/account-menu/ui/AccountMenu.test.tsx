import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice' }

async function openAccountPopover(user: ReturnType<typeof userEvent.setup>) {
  await user.click(await screen.findByRole('button', { name: '내 계정' }))
  return screen.findByRole('dialog', { name: '내 계정' })
}

describe('AccountMenu', () => {
  it('shows a profile icon, the id, and the logout action behind one control', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', { user: { ...USER, plan: ProtoPlan.FREE } })

    const trigger = await screen.findByRole('button', { name: '내 계정' })
    expect(trigger).toHaveClass('rounded-full', 'size-11')
    expect(trigger.querySelector('svg')).toHaveClass('lucide-user-round', 'size-5')
    expect(trigger).not.toHaveTextContent('A')
    // The header itself carries no id text or logout button until the popover opens.
    expect(screen.queryByText('alice')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '로그아웃' })).not.toBeInTheDocument()

    const panel = await openAccountPopover(user)
    expect(within(panel).getByText('로그인한 계정')).toBeInTheDocument()
    expect(within(panel).getByText('alice')).toBeInTheDocument()
    expect(within(panel).getByRole('button', { name: '로그아웃' })).toBeInTheDocument()
  })

  // Plan 17 A9: the shell shows the tier, and every number behind it comes from GetMyPlan —
  // nothing about a limit is known to the client until the server says it.
  it('fetches the tier and the three meters only when the popover opens', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.FREE },
      calls,
      plans: {
        plan: ProtoPlan.FREE,
        limits: {
          dailyJobStarts: 10,
          dailyBudgetMicrousd: 100_000n,
          monthlyBudgetMicrousd: 2_000_000n,
        },
        usage: {
          jobsStartedToday: 4,
          costTodayMicrousd: 70_000n,
          costMonthMicrousd: 350_000n,
          dayResetsAt: '2026-09-01T15:00:00Z',
          monthResetsAt: '2026-09-30T15:00:00Z',
        },
      },
    })

    await screen.findByRole('button', { name: '내 계정' })
    // The panel is what costs a request, so nothing is asked until it is opened.
    expect(calls).not.toContain('GetMyPlan')

    const panel = await openAccountPopover(user)
    expect(calls).toContain('GetMyPlan')
    expect(within(panel).getByText('Free')).toBeInTheDocument()

    // Micro-USD is the wire unit; the reader sees money.
    expect(await within(panel).findByText('US$0.07 / US$0.10')).toBeInTheDocument()
    expect(within(panel).getByText('4 / 10')).toBeInTheDocument()
    expect(within(panel).getByText('US$0.35 / US$2.00')).toBeInTheDocument()
    expect(within(panel).getByText('플랜은 운영자가 지정해요.')).toBeInTheDocument()

    // The meter is never the only signal: each one carries its own figures as text.
    const meters = within(panel).getAllByRole('meter')
    expect(meters).toHaveLength(3)
    expect(meters[0]).toHaveAttribute('aria-valuetext', '4 / 10')
  })

  // Plan 17 A9: an unlimited axis is stated, not drawn as an empty bar that reads as "none left".
  it('states unlimited for the operator tier and links the admin screen', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.MASTER },
      plans: {
        plan: ProtoPlan.MASTER,
        usage: { jobsStartedToday: 12, costTodayMicrousd: 4_120_000n },
      },
    })

    const panel = await openAccountPopover(user)
    expect(await within(panel).findByText('12 · 제한 없음')).toBeInTheDocument()
    expect(within(panel).getByText('US$4.12 · 제한 없음')).toBeInTheDocument()
    expect(within(panel).queryAllByRole('meter')).toHaveLength(0)
    expect(within(panel).getByRole('link', { name: '계정 관리' })).toHaveClass('min-h-11')
  })

  it('does not offer the admin screen to a tier that cannot use it', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.MAX },
      plans: { plan: ProtoPlan.MAX },
    })

    const panel = await openAccountPopover(user)
    await within(panel).findByText('오늘 사용량')
    expect(within(panel).queryByRole('link', { name: '계정 관리' })).not.toBeInTheDocument()
  })

  it('says so when the usage cannot be read', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, planFails: true },
    })

    await openAccountPopover(user)
    expect(await screen.findByText('사용량을 불러오지 못했어요.')).toBeInTheDocument()
  })
})
