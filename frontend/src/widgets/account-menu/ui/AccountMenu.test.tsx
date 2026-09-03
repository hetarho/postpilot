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

  // Change 19 A10: the shell shows the tier, and every number behind it comes from
  // GetMyPlan — nothing about a grant is known to the client until the server says it.
  it('fetches the balance and its lots only when the popover opens', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.FREE },
      calls,
      plans: {
        plan: ProtoPlan.FREE,
        balance: {
          credits: 62,
          unlimited: false,
          monthlyGrant: 50,
          renewsAt: '2026-09-30T15:00:00Z',
          lots: [
            { kind: 'monthly', granted: 50, remaining: 12, expiresAt: '2026-09-30T15:00:00Z' },
            { kind: 'bonus', granted: 50, remaining: 50 },
          ],
        },
      },
    })

    await screen.findByRole('button', { name: '내 계정' })
    // The panel is what costs a request, so nothing is asked until it is opened.
    expect(calls).not.toContain('GetMyPlan')

    const panel = await openAccountPopover(user)
    expect(calls).toContain('GetMyPlan')
    expect(within(panel).getByText('Free')).toBeInTheDocument()

    expect(await within(panel).findByText('62 크레딧')).toBeInTheDocument()
    // The lots behind the total: one lapses at the boundary, one does not, and a single
    // number cannot say that.
    expect(within(panel).getByText('12 / 50 크레딧')).toBeInTheDocument()
    expect(within(panel).getByText('50 / 50 크레딧')).toBeInTheDocument()
    expect(within(panel).getByText('플랜은 운영자가 지정해요.')).toBeInTheDocument()
    expect(within(panel).getByRole('link', { name: '플랜 보기' })).toBeInTheDocument()

    // The meter is never the only signal: it carries its own figure as text.
    const meter = within(panel).getByRole('meter')
    expect(meter).toHaveAttribute('aria-valuetext', '62 크레딧')
  })

  // An unlimited account is stated, not drawn as an empty bar that reads as "none left".
  it('states unlimited for the operator tier and links the admin screen', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.MASTER },
      plans: { plan: ProtoPlan.MASTER, balance: { unlimited: true } },
    })

    const panel = await openAccountPopover(user)
    expect(await within(panel).findByText('제한 없음')).toBeInTheDocument()
    expect(within(panel).queryAllByRole('meter')).toHaveLength(0)
    expect(within(panel).getByRole('link', { name: '운영 관리' })).toHaveClass('min-h-11')
  })

  it('does not offer the admin screen to a tier that cannot use it', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.MAX },
      plans: { plan: ProtoPlan.MAX },
    })

    const panel = await openAccountPopover(user)
    await within(panel).findByText('남은 크레딧')
    expect(within(panel).queryByRole('link', { name: '운영 관리' })).not.toBeInTheDocument()
  })

  it('says so when the balance cannot be read', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, planFails: true },
    })

    await openAccountPopover(user)
    expect(await screen.findByText('크레딧을 불러오지 못했어요.')).toBeInTheDocument()
  })
})
