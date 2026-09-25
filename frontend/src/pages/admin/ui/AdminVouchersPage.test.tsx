import { afterEach, describe, expect, it, vi } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan, ProtoVoucherState } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeVoucherIssue, FakeVoucherOptions } from '@/test/vouchers'

const MASTER = { id: 'root', plan: ProtoPlan.MASTER }

const SEEDS: FakeVoucherOptions['vouchers'] = [
  { id: 'v-open', token: 'tok-open', credits: 330, validityDays: 30 },
  {
    id: 'v-spent',
    token: 'tok-spent',
    credits: 1150,
    state: ProtoVoucherState.REDEEMED,
    sale: { amountKrw: 14900, payerName: '김민수' },
    message: '파일럿 감사합니다',
    redeemedBy: 'alice@example.com',
    redeemedAt: '2026-09-26T03:00:00Z',
    remainingCredits: 700,
    creditsExpireAt: '2026-10-26T03:00:00Z',
  },
  {
    id: 'v-revoked',
    token: 'tok-revoked',
    credits: 50,
    validityDays: 7,
    state: ProtoVoucherState.REVOKED,
  },
]

function renderTab(vouchers: FakeVoucherOptions = {}) {
  return renderAppAt('/admin/vouchers', {
    user: MASTER,
    vouchers: { vouchers: SEEDS, ...vouchers },
  })
}

function rowOf(text: string): HTMLElement {
  const row = screen.getByText(text).closest('li')
  if (!row) throw new Error(`no row for ${text}`)
  return row
}

describe('AdminVouchersPage', () => {
  afterEach(() => vi.restoreAllMocks())

  it('is the fourth admin tab', async () => {
    renderTab()
    const tab = await screen.findByRole('link', { name: /이용권/ })
    expect(tab).toHaveAttribute('href', '/admin/vouchers')
  })

  // GIFT-14: the list is the pilot's ledger — sale, redeemer, what is left — and a link to copy
  // only while it can still be redeemed.
  it('lists every voucher with its sale, state and remaining credits', async () => {
    renderTab()

    const open = rowOf(
      await screen.findByText('330 크레딧 · 30일').then((node) => node.textContent ?? ''),
    )
    expect(within(open).getByText('받기 전')).toBeInTheDocument()
    expect(within(open).getByText(/선물$/)).toBeInTheDocument()
    expect(within(open).getByRole('textbox', { name: '선물 링크' })).toHaveValue(
      `${window.location.origin}/gift/tok-open`,
    )

    const spent = rowOf('1150 크레딧 · 30일')
    expect(within(spent).getByText('받음')).toBeInTheDocument()
    expect(within(spent).getByText(/판매 14,900원 · 김민수$/)).toBeInTheDocument()
    expect(within(spent).getByText('파일럿 감사합니다')).toBeInTheDocument()
    expect(within(spent).getByText(/^alice@example.com · .* 받음$/)).toBeInTheDocument()
    expect(within(spent).getByText(/^남은 크레딧 700 · .*까지$/)).toBeInTheDocument()
    expect(within(spent).queryByRole('textbox')).not.toBeInTheDocument()

    const revoked = rowOf('50 크레딧 · 7일')
    expect(within(revoked).getByText('취소됨')).toBeInTheDocument()
    expect(within(revoked).queryByRole('button', { name: '발급 취소' })).not.toBeInTheDocument()
  })

  it('issues a sold voucher from a preset and shows the link to copy', async () => {
    const user = userEvent.setup()
    const issueRequests: FakeVoucherIssue[] = []
    renderTab({ issueRequests })

    await user.click(await screen.findByRole('tab', { name: 'pro · 1150 크레딧 · 30일' }))
    await user.type(screen.getByLabelText('받은 금액(원)'), '14900')
    expect(screen.getByLabelText('받은 금액(원)')).toHaveValue('14,900')
    await user.type(screen.getByLabelText('입금자'), '김민수')
    await user.type(screen.getByLabelText('메시지'), '감사합니다')
    await user.click(screen.getByRole('button', { name: '발급' }))

    expect(
      await screen.findByText('이용권을 발급했어요. 링크를 복사해 보내 주세요.'),
    ).toBeInTheDocument()
    expect(issueRequests).toEqual([
      {
        credits: 1150,
        validityDays: 30,
        message: '감사합니다',
        sale: { amountKrw: 14900, payerName: '김민수' },
      },
    ])
    const issuedSection = screen
      .getByRole('heading', { name: '이용권 발급' })
      .closest('section') as HTMLElement
    expect(within(issuedSection).getByRole('textbox', { name: '선물 링크' })).toHaveValue(
      `${window.location.origin}/gift/issued-token-4`,
    )
  })

  it('issues a given voucher with custom numbers', async () => {
    const user = userEvent.setup()
    const issueRequests: FakeVoucherIssue[] = []
    renderTab({ issueRequests })

    await user.click(await screen.findByRole('tab', { name: '직접 입력' }))
    await user.type(screen.getByLabelText('크레딧'), '500')
    await user.type(screen.getByLabelText('사용 기간(일)'), '14')
    await user.click(screen.getByRole('tab', { name: '선물' }))
    await user.click(screen.getByRole('button', { name: '발급' }))

    expect(
      await screen.findByText('이용권을 발급했어요. 링크를 복사해 보내 주세요.'),
    ).toBeInTheDocument()
    expect(issueRequests).toEqual([
      { credits: 500, validityDays: 14, message: '', sale: undefined },
    ])
  })

  it('says what is missing before it asks the server', async () => {
    const user = userEvent.setup()
    const issueRequests: FakeVoucherIssue[] = []
    renderTab({ issueRequests })

    await user.type(await screen.findByLabelText('받은 금액(원)'), '14900')
    await user.click(screen.getByRole('button', { name: '발급' }))

    expect(await screen.findByText('입금자를 40자 이내로 입력해 주세요.')).toBeInTheDocument()
    expect(issueRequests).toEqual([])
  })

  it('reports a copied link and a clipboard that refused', async () => {
    const user = userEvent.setup()
    const writeText = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error('denied'))
    vi.spyOn(navigator, 'clipboard', 'get').mockReturnValue({ writeText } as unknown as Clipboard)
    renderTab()

    const open = rowOf(
      await screen.findByText('330 크레딧 · 30일').then((node) => node.textContent ?? ''),
    )
    await user.click(within(open).getByRole('button', { name: '링크 복사' }))
    expect(await within(open).findByText('복사했어요')).toBeInTheDocument()
    expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/gift/tok-open`)

    await user.click(within(open).getByRole('button', { name: '링크 복사' }))
    expect(
      await within(open).findByText('복사하지 못했어요. 링크를 직접 선택해 복사해 주세요.'),
    ).toBeInTheDocument()
  })

  // GIFT-10: the confirm says what revoking does to this voucher, and the row changes in place.
  it('revokes a redeemed voucher after confirming, and cancelling changes nothing', async () => {
    const user = userEvent.setup()
    const revokeRequests: string[] = []
    renderTab({ revokeRequests })

    const spent = rowOf(
      await screen.findByText('1150 크레딧 · 30일').then((node) => node.textContent ?? ''),
    )
    await user.click(within(spent).getByRole('button', { name: '발급 취소' }))
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText(/남은 700 크레딧을 더 이상 쓸 수 없게 돼요/),
    ).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '취소' }))
    expect(revokeRequests).toEqual([])

    await user.click(within(spent).getByRole('button', { name: '발급 취소' }))
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: '발급 취소' }),
    )

    expect(await within(rowOf('1150 크레딧 · 30일')).findByText('취소됨')).toBeInTheDocument()
    expect(revokeRequests).toEqual(['v-spent'])
  })

  it('reports a list that could not be read', async () => {
    renderTab({ listUnavailable: true })
    expect(await screen.findByText('이용권을 불러오지 못했어요.')).toBeInTheDocument()
  })

  it('says so when nothing has been issued', async () => {
    renderAppAt('/admin/vouchers', { user: MASTER, vouchers: { vouchers: [] } })
    expect(await screen.findByText('아직 발급한 이용권이 없어요.')).toBeInTheDocument()
  })
})
