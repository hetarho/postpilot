import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }

describe('the admin screen', () => {
  // A10: the operator sees every account and changes a tier from here.
  it('lists accounts and changes one account’s tier', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/admin', {
      user: MASTER,
      calls,
      plans: {
        plan: ProtoPlan.MASTER,
        accounts: [
          { id: 'root', plan: ProtoPlan.MASTER },
          { id: 'alice', plan: ProtoPlan.FREE },
        ],
      },
    })

    expect(await screen.findByRole('heading', { name: '계정 관리' })).toBeInTheDocument()
    const alice = await screen.findByRole('combobox', { name: 'alice 계정의 플랜' })
    expect(alice).toHaveValue('free')

    await user.selectOptions(alice, 'max')
    expect(calls).toContain('SetUserPlan')
    expect(await screen.findByRole('combobox', { name: 'alice 계정의 플랜' })).toHaveValue('max')
  })

  // A10: the last-master guard lives on the server, so the screen renders its refusal rather
  // than predicting one — and the row keeps the tier the database still has.
  it('reports the server’s refusal to demote the last master', async () => {
    const user = userEvent.setup()
    renderAppAt('/admin', {
      user: MASTER,
      plans: {
        plan: ProtoPlan.MASTER,
        accounts: [{ id: 'root', plan: ProtoPlan.MASTER }],
        setPlanFails: true,
      },
    })

    const root = await screen.findByRole('combobox', { name: 'root 계정의 플랜' })
    await user.selectOptions(root, 'basic')

    const alert = await screen.findByRole('alert')
    expect(
      within(alert).getByText('마지막 운영자 계정은 다른 플랜으로 바꿀 수 없어요.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'root 계정의 플랜' })).toHaveValue('master')
  })

  // A10: a tier that cannot use the screen never reaches it. The server refuses the two
  // procedures anyway — this only keeps a signed-in visitor from landing on a dead page.
  it('sends a non-operator back to the app', async () => {
    renderAppAt('/admin', {
      user: { id: 'alice', plan: ProtoPlan.MAX },
      plans: { plan: ProtoPlan.MAX },
    })

    expect(await screen.findByRole('heading', { name: '내 글' })).toBeInTheDocument()
    expect(screen.queryByRole('heading', { name: '계정 관리' })).not.toBeInTheDocument()
  })
})
