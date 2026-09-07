import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ProtoPlan, ProtoTerm } from '@/shared/api'
import { renderAppAt } from '@/test/app'

vi.mock('@/shared/config', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/shared/config')>()),
  TOSS_CLIENT_KEY: 'test-client-key',
}))

const verifiedUser = {
  id: 'alice',
  plan: ProtoPlan.FREE,
  email: 'alice@example.com',
  emailVerified: true,
}

describe('BillingCheckoutPage', () => {
  it('quotes and applies an active subscriber upgrade immediately, then refreshes plan data', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const changeRequests: Array<{ plan: ProtoPlan; term: ProtoTerm }> = []
    const { router } = renderAppAt('/billing/checkout?tier=max', {
      user: { ...verifiedUser, plan: ProtoPlan.PRO },
      calls,
      plans: { plan: ProtoPlan.PRO },
      billing: {
        subscription: true,
        paymentMethod: true,
        subscriptionState: { plan: ProtoPlan.PRO, term: ProtoTerm.MONTHLY },
        changeRequests,
      },
    })

    await waitFor(() => expect(calls).toContain('QuoteChange'))
    expect(await screen.findByText('$5.00 · 7,000원')).toBeInTheDocument()
    expect(screen.getByText('지금 결제되는 업그레이드 금액입니다.')).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '연간' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '지금 결제하고 업그레이드' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/billing'))
    expect(changeRequests).toEqual([{ plan: ProtoPlan.MAX, term: ProtoTerm.MONTHLY }])
    expect(calls.filter((call) => call === 'GetMyPlan').length).toBeGreaterThan(1)
    expect(await screen.findByText('Max 플랜으로 업그레이드했습니다.')).toBeInTheDocument()
  })

  it('quotes monthly and annual terms from the server and starts the selected subscription', async () => {
    const user = userEvent.setup()
    const subscribeRequests: Array<{ plan: ProtoPlan; term: ProtoTerm }> = []
    const { router } = renderAppAt('/billing/checkout?tier=pro', {
      user: verifiedUser,
      plans: { plan: ProtoPlan.FREE },
      billing: { paymentMethod: true, subscribeRequests },
    })

    expect(await screen.findByRole('heading', { name: 'Pro' })).toBeInTheDocument()
    expect(screen.getByText('매달 575 크레딧')).toBeInTheDocument()
    expect(screen.getByText('$5.00 · 7,000원')).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '연간' }))
    expect(screen.getByText('12개월에 10개월 요금')).toBeInTheDocument()
    expect(await screen.findByText('$50.00 · 70,000원')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '결제하고 구독하기' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/billing'))
    expect(subscribeRequests).toEqual([{ plan: ProtoPlan.PRO, term: ProtoTerm.ANNUAL }])
    expect(await screen.findByText('Pro 구독을 시작했습니다.')).toBeInTheDocument()
  })

  it('offers card registration that returns to this checkout', async () => {
    renderAppAt('/billing/checkout?tier=basic', {
      user: verifiedUser,
      plans: { plan: ProtoPlan.FREE },
    })

    expect(await screen.findByText('구독하려면 먼저 카드를 등록해 주세요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '카드 등록' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '결제하고 구독하기' })).not.toBeInTheDocument()
  })

  it.each([
    ['CHARGE_FAILED', '결제를 완료하지 못했어요. 결제 수단을 확인하고 다시 시도해 주세요.'],
    ['PAYMENT_METHOD_REQUIRED', '먼저 결제 수단을 등록해 주세요.'],
    ['EMAIL_VERIFICATION_REQUIRED', '결제 수단을 등록하려면 이메일 인증이 필요해요.'],
  ] as const)('renders the localized %s refusal', async (reason, message) => {
    const user = userEvent.setup()
    renderAppAt('/billing/checkout?tier=max', {
      user: verifiedUser,
      plans: { plan: ProtoPlan.FREE },
      billing: { paymentMethod: true, subscribeFailure: reason },
    })

    await user.click(await screen.findByRole('button', { name: '결제하고 구독하기' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(message)
  })

  it('refuses an invalid or missing paid rung', async () => {
    renderAppAt('/billing/checkout?tier=free', { user: verifiedUser })
    expect(await screen.findByRole('alert')).toHaveTextContent(
      '구독할 유료 플랜을 다시 선택해 주세요.',
    )
    expect(screen.queryByRole('button', { name: '결제하고 구독하기' })).not.toBeInTheDocument()
  })
})
