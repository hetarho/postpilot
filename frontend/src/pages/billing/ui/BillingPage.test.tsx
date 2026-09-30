import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { ProtoPlan, ProtoTerm } from '@/shared/api'
import { renderAppAt } from '@/test/app'

afterEach(() => initializeI18n('ko'))

describe('BillingPage', () => {
  it('quotes and confirms a fixed credit pack, then refreshes billing and balance', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const purchaseRequests: Array<number | string> = []
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      plans: { plan: ProtoPlan.PRO },
      calls,
      billing: { paymentMethod: true, subscription: true, purchaseRequests },
    })

    expect(await screen.findByRole('radio', { name: '1000 크레딧 · 3,000원' })).toBeChecked()
    expect(screen.getByRole('radio', { name: '1000 크레딧 · 3,000원' })).toBeChecked()
    await user.click(screen.getByRole('button', { name: '크레딧 구매' }))
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveTextContent('1000 크레딧에 3,000원이 결제됩니다')
    expect(dialog).toHaveTextContent('만료되지 않습니다')
    expect(dialog).toHaveTextContent('마지막으로 차감됩니다')
    await user.click(within(dialog).getByRole('button', { name: '크레딧 구매' }))

    await waitFor(() => expect(purchaseRequests).toEqual(['pack-1000']))
    expect(await screen.findByText('1000 크레딧을 구매했습니다.')).toBeInTheDocument()
    expect(calls.filter((call) => call === 'GetMyBilling').length).toBeGreaterThan(1)
    expect(calls.filter((call) => call === 'GetMyPlan').length).toBeGreaterThan(1)
  })

  it('submits a reviewed refund request for a charged payment and shows pending state', async () => {
    const user = userEvent.setup()
    const refundRequests: string[] = []
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true, refundRequests },
    })

    expect(
      await screen.findByText(/사용했거나 7일이 지난 결제도 요청할 수 있으며/),
    ).toBeInTheDocument()
    await user.click(screen.getByRole('combobox', { name: '결제' }))
    await user.click(screen.getByRole('option', { name: /sub-1/ }))
    await user.type(screen.getByRole('textbox', { name: '요청 사유' }), '결제 검토를 부탁합니다')
    await user.click(screen.getByRole('button', { name: '환불 요청하기' }))
    await waitFor(() => expect(refundRequests).toEqual(['sub-1']))
    expect(await screen.findByText('환불 요청이 접수되었습니다.')).toBeInTheDocument()
    expect(await screen.findByText(/심사 대기 · 결제 검토를 부탁합니다/)).toBeInTheDocument()
  })

  it('keeps a failed refund request visible as a form error', async () => {
    const user = userEvent.setup()
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true, refundFailure: 'REFUND_FAILED' },
    })

    await user.click(await screen.findByRole('combobox', { name: '결제' }))
    await user.click(screen.getByRole('option', { name: /sub-1/ }))
    await user.type(screen.getByRole('textbox', { name: '요청 사유' }), '실패 확인')
    await user.click(screen.getByRole('button', { name: '환불 요청하기' }))
    expect(
      await screen.findByText('요청을 접수하지 못했습니다. 잠시 후 다시 시도해 주세요.'),
    ).toBeInTheDocument()
  })

  it('localizes a failed purchase charge in its confirmation dialog', async () => {
    const user = userEvent.setup()
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      plans: { plan: ProtoPlan.PRO },
      billing: { paymentMethod: true, subscription: true, purchaseFailure: 'CHARGE_FAILED' },
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
    expect(headings).toEqual([
      '혜택과 잔액',
      '구독',
      '결제 수단',
      '결제 및 지급 기록',
      '크레딧 구매',
      '환불 요청',
    ])
    expect(screen.getByRole('link', { name: '플랜 보기' })).toHaveAttribute('href', '/plans')
    expect(screen.getByText('등록된 결제 수단이 없습니다.')).toBeInTheDocument()
    expect(screen.getByText('아직 결제 기록이 없습니다.')).toBeInTheDocument()
    expect(screen.getByText('아직 구매한 크레딧이 없습니다.')).toBeInTheDocument()
    expect(calls.filter((call) => call === 'GetMyBilling')).toHaveLength(1)
    expect(screen.getByRole('button', { name: '환불 요청하기' })).toBeDisabled()
  })

  // BILL-20: a master account is never charged, so the screen offers no plan link, subscription
  // action or credit pack — refunds and the payment method stay.
  it('shows operator coverage to a master account with no payment entry', async () => {
    renderAppAt('/billing', {
      user: { id: 'root', plan: ProtoPlan.MASTER },
      plans: { plan: ProtoPlan.MASTER },
    })

    expect(
      await screen.findByText('운영자 계정이라 결제 없이 모든 기능을 써요.'),
    ).toBeInTheDocument()
    expect(screen.queryByText('활성 구독이 없습니다.')).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '플랜 보기' })).not.toBeInTheDocument()
    const headings = screen
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent)
    expect(headings).not.toContain('크레딧 구매')
    expect(headings).toContain('결제 수단')
    expect(headings).toContain('환불 요청')
  })

  // BILL-20: a subscription held from before the promotion ends at its term, uncharged, so its
  // date reads as the coverage end and it carries no cancel, resume or change action.
  it('shows a master account’s held subscription as ending without renewal', async () => {
    renderAppAt('/billing', {
      user: { id: 'root', plan: ProtoPlan.MASTER },
      plans: { plan: ProtoPlan.MASTER },
      billing: { populated: true },
    })

    expect(await screen.findByText('유료 이용 종료일')).toBeInTheDocument()
    expect(screen.queryByText('다음 실제 결제일')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '구독 해지' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '연간으로 바꾸기' })).not.toBeInTheDocument()
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

  it('renders active subscription clocks and KRW history', async () => {
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: { populated: true },
    })

    expect((await screen.findAllByText('Pro')).length).toBeGreaterThan(0)
    expect(screen.getByText('월간')).toBeInTheDocument()
    expect(screen.getByText('매월 8일')).toBeInTheDocument()
    expect(await screen.findByText('다음 실제 결제일')).toBeInTheDocument()
    const history = screen.getByRole('heading', { name: '결제 및 지급 기록' }).parentElement
    const rows = within(history as HTMLElement).getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(rows[0]).toHaveTextContent('플랜 변경')
    expect(rows[1]).toHaveTextContent('결제')
    expect(rows[1]).toHaveTextContent('7,000원')
    expect(rows[1]).not.toHaveTextContent('$')
  })

  it('separates annual payment, monthly benefit and daily reset at month end', async () => {
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      plans: {
        plan: ProtoPlan.PRO,
        balance: {
          credits: 70,
          dailyGrant: 85,
          monthlyBonus: 1070,
          dailyResetsAt: '2026-10-01T02:00:00Z',
          bonusResetsAt: '2026-10-31T14:30:00Z',
          lots: [
            { kind: 'daily', granted: 85, remaining: 10, expiresAt: '2026-10-01T02:00:00Z' },
            { kind: 'monthly', granted: 1070, remaining: 40, expiresAt: '2026-10-31T14:30:00Z' },
            { kind: 'purchased', granted: 20, remaining: 20 },
          ],
        },
        serverExportWindow: {
          coverageId: 'paid:alice',
          endsAt: '2026-10-31T14:30:00Z',
          allowance: 15,
          remaining: 12,
        },
      },
      billing: {
        subscription: true,
        subscriptionState: {
          plan: ProtoPlan.PRO,
          term: ProtoTerm.ANNUAL,
          anchorAt: '2026-09-30T14:30:00Z',
          termEnd: '2027-09-30T14:30:00Z',
          nextGrantAt: '2026-10-31T14:30:00Z',
        },
      },
    })
    expect(await screen.findByText('사용 가능 70 크레딧')).toBeInTheDocument()
    expect(screen.getByText('일일 지급 · 다음 지급')).toBeInTheDocument()
    expect(screen.getByText('월 보너스 · 다음 갱신')).toBeInTheDocument()
    expect(screen.getByText('다음 실제 결제')).toBeInTheDocument()
    expect(screen.getByText(/서버 내보내기 12 \/ 15회/)).toBeInTheDocument()
    expect(screen.getByText(/구매 · 20 \/ 20/)).toBeInTheDocument()
  })

  it('keeps a free account’s retained purchase visible while blocking a new pack', async () => {
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      plans: {
        plan: ProtoPlan.FREE,
        balance: { credits: 100, lots: [{ kind: 'purchased', granted: 100, remaining: 100 }] },
      },
      billing: { paymentMethod: true },
    })
    expect(await screen.findByText('사용 가능 100 크레딧')).toBeInTheDocument()
    expect(screen.getByText(/구매 · 100 \/ 100/)).toBeInTheDocument()
    expect(screen.getByText('크레딧 구매는 활성 유료 구독에서만 가능해요.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '크레딧 구매' })).toBeDisabled()
  })

  // QUOTA-65: a subscriber's billing screen shows credits and KRW prices, never the rate
  // behind credits.
  it('shows a subscriber no exchange rate', async () => {
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      plans: { plan: ProtoPlan.PRO },
      billing: { subscription: true },
    })
    expect(await screen.findByRole('heading', { name: '혜택과 잔액' })).toBeInTheDocument()
    expect(screen.queryByText(/환율|원\/USD/)).not.toBeInTheDocument()
  })

  it('distinguishes the temporary official reference from its applied rate for the operator', async () => {
    renderAppAt('/billing', {
      user: { id: 'root', plan: ProtoPlan.MASTER },
      plans: {
        plan: ProtoPlan.MASTER,
        fxRate: {
          source: 'BOK',
          publicationDate: '2026-09-29',
          referenceE4: 14123400n,
          appliedE4: 14200000n,
          temporary: true,
        },
      },
      billing: { subscription: true },
    })
    expect(
      await screen.findByText(/기준 환율 1,412.34원\/USD · 적용 환율 1,420원\/USD/),
    ).toBeInTheDocument()
    expect(screen.getByText(/임시 적용 중/)).toBeInTheDocument()
  })

  it('explains retained credits and FX recovery in English without promising a purchase', async () => {
    initializeI18n('en')
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      plans: {
        plan: ProtoPlan.FREE,
        fxUnavailable: true,
        balance: {
          credits: 30,
          lots: [{ kind: 'purchased', granted: 30, remaining: 30 }],
        },
      },
      billing: { paymentMethod: true },
    })
    expect(await screen.findByText('30 spendable credits')).toBeInTheDocument()
    expect(screen.getByText(/Retained credits do not unlock paid models/)).toBeInTheDocument()
    expect(
      screen.getByText(
        'Paid AI work cannot start right now. Free-model work and payments continue.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByText(/exchange rate|official rate|KRW\/USD/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Buy credits' })).toBeDisabled()
  })

  // BILL-15, F118: a refund row is labelled as a refund, not as a generic subscription event.
  it('labels a refund row in the history', async () => {
    renderAppAt('/billing', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      billing: {
        populated: true,
        extraHistory: [{ id: 3n, kind: 'refund', createdAt: '2026-09-09T00:00:00Z', krw: 7000n }],
      },
    })

    const heading = await screen.findByRole('heading', { name: '결제 및 지급 기록' })
    const rows = within(heading.parentElement as HTMLElement).getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent('환불')
    expect(rows[0]).not.toHaveTextContent('구독 기록')
  })
})
