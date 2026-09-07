import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

describe('BillingPage', () => {
  it('renders the empty money surface in contract order', async () => {
    const calls: string[] = []
    renderAppAt('/billing', { user: { id: 'alice', plan: ProtoPlan.FREE }, calls })

    expect(await screen.findByRole('heading', { name: '결제 관리' })).toBeInTheDocument()
    expect(await screen.findByText('활성 구독이 없습니다.')).toBeInTheDocument()
    const headings = screen
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent)
    expect(headings).toEqual(['구독', '결제 수단', '결제 및 지급 기록', '크레딧 구매'])
    expect(screen.getByRole('link', { name: '플랜 보기' })).toHaveAttribute('href', '/plans')
    expect(screen.getByText('등록된 결제 수단이 없습니다.')).toBeInTheDocument()
    expect(screen.getByText('아직 결제 기록이 없습니다.')).toBeInTheDocument()
    expect(screen.getByText('아직 구매한 크레딧이 없습니다.')).toBeInTheDocument()
    expect(calls.filter((call) => call === 'GetMyBilling')).toHaveLength(1)
    expect(document.querySelectorAll('form, input')).toHaveLength(0)
  })

  it('renders the active-renewal refusal beside the remove action', async () => {
    const user = userEvent.setup()
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true, removeFailure: 'SUBSCRIPTION_NEEDS_METHOD' },
    })

    expect(await screen.findByText('11 1234')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '카드 삭제' }))
    const dialog = screen.getByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '카드 삭제' }))

    expect(
      await screen.findByText(
        '자동 갱신 중인 구독에는 결제 수단이 필요해요. 먼저 구독을 취소해 주세요.',
      ),
    ).toBeInTheDocument()
  })
})
