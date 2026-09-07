import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { ProtoPlan, ProtoTerm } from '@/shared/api'
import { renderAppAt } from '@/test/app'

describe('BillingPage', () => {
  it('quotes and confirms an at-par credit purchase, then refreshes billing and balance', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const purchaseRequests: number[] = []
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      calls,
      billing: { paymentMethod: true, purchaseRequests },
    })

    expect(await screen.findByText('500 크레딧 · 7,000원')).toBeInTheDocument()
    expect(screen.getByRole('spinbutton', { name: '구매 금액' })).toHaveTextContent('$5')
    await user.click(screen.getByRole('button', { name: '크레딧 구매' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('$5.00 · 500 크레딧 · 7,000원')
    expect(dialog).toHaveTextContent('만료되지 않습니다')
    expect(dialog).toHaveTextContent('마지막으로 차감됩니다')
    await user.click(within(dialog).getByRole('button', { name: '크레딧 구매' }))

    await waitFor(() => expect(purchaseRequests).toEqual([500]))
    expect(await screen.findByText('500 크레딧을 구매했습니다.')).toBeInTheDocument()
    expect(calls.filter((call) => call === 'GetMyBilling').length).toBeGreaterThan(1)
    expect(calls.filter((call) => call === 'GetMyPlan').length).toBeGreaterThan(1)
  })

  it('shows the seven-day rule and refunds only server-marked purchases', async () => {
    const user = userEvent.setup()
    const refundRequests: string[] = []
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true, refundRequests },
    })

    await user.click(await screen.findByRole('button', { name: '환불' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('구매 후 7일 안에')
    expect(dialog).toHaveTextContent('하나도 사용하지 않은 경우')
    await user.click(within(dialog).getByRole('button', { name: '환불' }))

    await waitFor(() => expect(refundRequests).toEqual(['purchase-1']))
    expect(await screen.findByText('100 크레딧 구매를 환불했습니다.')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '환불' })).not.toBeInTheDocument()
  })

  it.each([
    ['PURCHASE_SPENT', '구매한 크레딧을 일부 사용해 환불할 수 없어요.'],
    ['REFUND_WINDOW_CLOSED', '구매 후 7일이 지나 환불할 수 없어요.'],
    ['REFUND_FAILED', '환불을 완료하지 못했어요. 잠시 후 다시 시도해 주세요.'],
  ] as const)('localizes %s in the refund dialog', async (failure, message) => {
    const user = userEvent.setup()
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true, refundFailure: failure },
    })

    await user.click(await screen.findByRole('button', { name: '환불' }))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: '환불' }))
    expect(await screen.findByText(message)).toBeInTheDocument()
  })

  it('localizes a failed purchase charge in its confirmation dialog', async () => {
    const user = userEvent.setup()
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      billing: { paymentMethod: true, purchaseFailure: 'CHARGE_FAILED' },
    })

    await user.click(await screen.findByRole('button', { name: '크레딧 구매' }))
    await user.click(
      within(screen.getByRole('dialog')).getByRole('button', { name: '크레딧 구매' }),
    )
    expect(
      await screen.findByText('결제를 완료하지 못했어요. 결제 수단을 확인하고 다시 시도해 주세요.'),
    ).toBeInTheDocument()
  })

  it('shows and cancels a scheduled subscription change', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      calls,
      billing: {
        populated: true,
        subscriptionState: {
          plan: ProtoPlan.PRO,
          term: ProtoTerm.MONTHLY,
          scheduledPlan: ProtoPlan.BASIC,
          scheduledTerm: ProtoTerm.MONTHLY,
        },
      },
    })

    expect(await screen.findByText(/Basic · 월간으로 변경 예정/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '예약 취소' }))
    await waitFor(() => expect(calls).toContain('CancelScheduledChange'))
    expect(await screen.findByRole('button', { name: '연간으로 바꾸기' })).toBeInTheDocument()
  })

  it('confirms cancellation without removing credits and can resume before term end', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      calls,
      billing: { populated: true },
    })

    await user.click(await screen.findByRole('button', { name: '구독 해지' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('남은 크레딧을 그대로 쓸 수 있습니다')
    await user.click(within(dialog).getByRole('button', { name: '구독 해지' }))
    expect(await screen.findByRole('button', { name: '해지 취소' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '해지 취소' }))
    await waitFor(() => expect(calls).toContain('ResumeSubscription'))
    expect(await screen.findByRole('button', { name: '구독 해지' })).toBeInTheDocument()
  })

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

  it('renders active subscription timing, a moving quote, and newest-first history', async () => {
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true },
    })

    expect(await screen.findByText('Pro')).toBeInTheDocument()
    expect(screen.getByText('월간')).toBeInTheDocument()
    expect(screen.getByText('매월 8일')).toBeInTheDocument()
    expect(await screen.findByText(/오늘 기준 약 7,000원 · 변동/)).toBeInTheDocument()
    const history = screen.getByRole('heading', { name: '결제 및 지급 기록' }).parentElement
    const rows = within(history as HTMLElement).getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('플랜 변경')
    expect(rows[1]).toHaveTextContent('결제')
    expect(rows[1]).toHaveTextContent('$5.00 · 7,000원')
    expect(rows[1]).toHaveTextContent('1달러당 1,400원')
  })
})
