import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { create } from '@bufbuild/protobuf'
import { myPlanQueryKey } from '@/entities/plan'
import { GetMyPlanResponseSchema, ProtoVoucherState } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const PENDING = 'postpilot.pendingGift'

describe('GiftPage', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  // GIFT-8: the page is public — a visitor with no account deep-links to it and sees what the
  // link holds, then gets the two ways in with this page as the destination.
  it('shows a redeemable voucher to an anonymous visitor and remembers the link', async () => {
    const calls: string[] = []
    const { router } = renderAppAt('/gift/tok-1', {
      calls,
      vouchers: {
        vouchers: [
          { token: 'tok-1', credits: 1150, validityDays: 30, message: '파일럿 감사합니다' },
        ],
      },
    })

    expect(await screen.findByRole('heading', { name: '1150 크레딧' })).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/gift/tok-1')
    expect(screen.getByText('파일럿 감사합니다')).toBeInTheDocument()
    expect(screen.getByText('받은 날부터 30일 동안 사용')).toBeInTheDocument()
    expect(screen.getByText(/까지 받을 수 있어요$/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '로그인하고 받기' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Fgift%2Ftok-1',
    )
    expect(screen.getByRole('link', { name: '가입하고 받기' })).toHaveAttribute(
      'href',
      '/signup?redirect=%2Fgift%2Ftok-1',
    )
    expect(screen.queryByRole('button', { name: '받기' })).not.toBeInTheDocument()
    expect(JSON.parse(localStorage.getItem(PENDING) ?? '{}').token).toBe('tok-1')
    // Opening the page never redeems it (GIFT-9).
    expect(calls).not.toContain('RedeemVoucher')
  })

  it('falls back to the default line when the voucher carries no message', async () => {
    renderAppAt('/gift/tok-1', { vouchers: { vouchers: [{ token: 'tok-1' }] } })
    expect(await screen.findByText('Postpilot 이용권이 도착했어요')).toBeInTheDocument()
  })

  // GIFT-9: signed in, one explicit action redeems it, and the balance the header reads is
  // marked stale so it shows the new credits.
  it('redeems for a signed-in account and refreshes the balance', async () => {
    const user = userEvent.setup()
    const redeemRequests: string[] = []
    localStorage.setItem(PENDING, JSON.stringify({ token: 'tok-1', savedAt: Date.now() }))
    const { queryClient, transport } = renderAppAt('/gift/tok-1', {
      user: { id: 'alice' },
      vouchers: { vouchers: [{ token: 'tok-1', credits: 330 }], redeemRequests },
    })
    queryClient.setQueryData(myPlanQueryKey(transport), create(GetMyPlanResponseSchema, {}))

    await user.click(await screen.findByRole('button', { name: '받기' }))

    expect(await screen.findByText(/^330 크레딧을 받았어요\./)).toBeInTheDocument()
    expect(redeemRequests).toEqual(['tok-1'])
    expect(screen.getByRole('link', { name: '시작하기' })).toHaveAttribute('href', '/posts')
    expect(queryClient.getQueryState(myPlanQueryKey(transport))?.isInvalidated).toBe(true)
    // The link this visitor was headed to is reached; nothing is left to hand back.
    expect(localStorage.getItem(PENDING)).toBeNull()
  })

  it('shows the state a refused redemption left the link in', async () => {
    const user = userEvent.setup()
    renderAppAt('/gift/tok-1', {
      user: { id: 'alice' },
      vouchers: { vouchers: [{ token: 'tok-1' }], redeemFailure: 'VOUCHER_REDEEMED' },
    })

    await user.click(await screen.findByRole('button', { name: '받기' }))

    expect(
      await screen.findByRole('heading', { name: '이미 받은 이용권이에요' }),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '받기' })).not.toBeInTheDocument()
  })

  it.each([
    [ProtoVoucherState.REDEEMED, '이미 받은 이용권이에요'],
    [ProtoVoucherState.EXPIRED, '받을 수 있는 기간이 지난 이용권이에요'],
    [ProtoVoucherState.REVOKED, '취소된 이용권이에요'],
  ])('renders a link that can no longer be redeemed (%s)', async (state, heading) => {
    localStorage.setItem(PENDING, JSON.stringify({ token: 'tok-1', savedAt: Date.now() }))
    renderAppAt('/gift/tok-1', { vouchers: { vouchers: [{ token: 'tok-1', state }] } })

    expect(await screen.findByRole('heading', { name: heading })).toBeInTheDocument()
    expect(screen.getByText('보내 준 분께 새 링크를 요청해 주세요.')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: '로그인하고 받기' })).not.toBeInTheDocument()
    await waitFor(() => expect(localStorage.getItem(PENDING)).toBeNull())
  })

  it('says so when the link names no voucher', async () => {
    renderAppAt('/gift/unknown', { vouchers: { vouchers: [] } })
    expect(
      await screen.findByRole('heading', { name: '이용권을 찾을 수 없어요' }),
    ).toBeInTheDocument()
  })

  it('reports a failed read with the shared failure message', async () => {
    renderAppAt('/gift/tok-1', {
      vouchers: { vouchers: [{ token: 'tok-1' }], getUnavailable: true },
    })
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '받기' })).not.toBeInTheDocument()
  })
})

// The email-verification detour loses the login redirect; the first signed-in screen hands
// the visitor back to the gift link they opened.
describe('pending gift hand-back', () => {
  beforeEach(() => localStorage.clear())
  afterEach(() => localStorage.clear())

  it('sends a fresh pending gift back to its page once', async () => {
    localStorage.setItem(PENDING, JSON.stringify({ token: 'tok-1', savedAt: Date.now() }))
    const { router } = renderAppAt('/posts', {
      user: { id: 'alice' },
      vouchers: { vouchers: [{ token: 'tok-1' }] },
    })

    await waitFor(() => expect(router.state.location.pathname).toBe('/gift/tok-1'))
    expect(await screen.findByRole('button', { name: '받기' })).toBeInTheDocument()
  })

  it('leaves a stale pending gift where it is', async () => {
    localStorage.setItem(
      PENDING,
      JSON.stringify({ token: 'tok-1', savedAt: Date.now() - 2 * 24 * 60 * 60 * 1000 }),
    )
    const { router } = renderAppAt('/posts', { user: { id: 'alice' } })

    await screen.findByRole('button', { name: '내 계정' })
    expect(router.state.location.pathname).toBe('/posts')
    expect(localStorage.getItem(PENDING)).toBeNull()
  })
})
