import { beforeEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan, ProtoTerm } from '@/shared/api'
import { CLIP_ESTIMATE_STORAGE_KEY, PLAN_ESTIMATE_STORAGE_KEY } from '../config'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice', plan: ProtoPlan.BASIC }

beforeEach(() => {
  localStorage.removeItem(PLAN_ESTIMATE_STORAGE_KEY)
  localStorage.removeItem(CLIP_ESTIMATE_STORAGE_KEY)
})

async function cards() {
  return within(await screen.findByRole('list')).getAllByRole('listitem')
}

describe('the published plans', () => {
  it('keeps upgrade, scheduled downgrade and cancellation in their billing paths', async () => {
    const user = userEvent.setup()
    const changeRequests: Array<{ plan: ProtoPlan; term: ProtoTerm }> = []
    renderAppAt('/plans', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      plans: { plan: ProtoPlan.PRO },
      billing: {
        subscription: true,
        subscriptionState: { plan: ProtoPlan.PRO, term: ProtoTerm.MONTHLY },
        changeRequests,
      },
    })
    const items = await cards()
    expect(
      within(items[0]).getByRole('link', { name: '구독 해지는 결제 관리에서' }),
    ).toHaveAttribute('href', '/billing')
    expect(within(items[4]).getByRole('link', { name: '업그레이드' })).toHaveAttribute(
      'href',
      '/billing/checkout?tier=max',
    )
    await user.click(within(items[2]).getByRole('button', { name: '다음 결제일부터' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '변경 예약' }))
    await waitFor(() =>
      expect(changeRequests).toEqual([{ plan: ProtoPlan.BASIC, term: ProtoTerm.MONTHLY }]),
    )
  })

  it('keeps the operator out of checkout and explains an empty balance', async () => {
    const { unmount } = renderAppAt('/plans', {
      user: { id: 'alice', plan: ProtoPlan.MASTER },
      plans: { plan: ProtoPlan.MASTER },
    })
    await cards()
    expect(screen.queryByRole('link', { name: '구독하기' })).not.toBeInTheDocument()
    expect(screen.getByText('운영자 계정이라 결제 없이 모든 기능을 써요.')).toBeInTheDocument()
    unmount()
    renderAppAt('/plans', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 0, unlimited: false } },
    })
    expect(await screen.findByText('크레딧이 부족해요')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '크레딧 구매' })).toHaveAttribute('href', '/billing')
  })

  it('shows five KRW offers with separate daily, bonus, export and model rights', async () => {
    renderAppAt('/plans', { user: USER, plans: { plan: ProtoPlan.BASIC } })
    const items = await cards()
    expect(items).toHaveLength(5)
    expect(screen.getByRole('list')).toHaveClass('md:grid-cols-2', 'xl:grid-cols-5')
    for (const [index, name, monthly, annual, daily, bonus, exports] of [
      [0, 'Free', 0, 0, 0, 0, 0],
      [1, 'Light', 1900, 19000, 15, 290, 2],
      [2, 'Basic', 4900, 49000, 45, 510, 6],
      [3, 'Pro', 9900, 99000, 85, 1070, 15],
      [4, 'Max', 29900, 299000, 235, 3170, 60],
    ] as const) {
      const card = within(items[index])
      expect(card.getByRole('heading', { name })).toBeInTheDocument()
      expect(card.getByText(`매일 ${daily} 크레딧 지급`)).toBeInTheDocument()
      expect(card.getByText(`매월 보너스 ${bonus} 크레딧`)).toBeInTheDocument()
      expect(card.getByText(`서버 내보내기 월 ${exports}회`)).toBeInTheDocument()
      expect(card.getByText('클립 원본·완성본 최대 60초')).toBeInTheDocument()
      if (monthly) {
        expect(card.getByText(`${monthly.toLocaleString('en-US')}원`)).toBeInTheDocument()
        expect(
          card.getByText(`연간 선결제 ${annual.toLocaleString('en-US')}원`),
        ).toBeInTheDocument()
        expect(annual).toBe(monthly * 10)
        expect(card.getByText(/약 16.7% 절약/)).toBeInTheDocument()
      } else {
        expect(card.getByText('무료 모델은 제공사 제한에 따라 사용')).toBeInTheDocument()
      }
    }
    expect(screen.getAllByText('가장 합리적')).toHaveLength(1)
    expect(within(items[3]).getByText('가장 합리적')).toBeInTheDocument()
    expect(within(items[2]).getByText('지금 쓰는 플랜')).toBeInTheDocument()
    expect(within(items[3]).getByRole('link', { name: '구독하기' })).toHaveAttribute(
      'href',
      '/billing/checkout?tier=pro',
    )
  })

  it('shows locked, eligible and missing model levels in order', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: {
        plan: ProtoPlan.BASIC,
        estimatorCombos: [
          { combo: 'value' },
          { combo: 'balanced' },
          { combo: 'premium' },
          { combo: 'top' },
        ],
      },
    })
    const items = await cards()
    expect(within(items[0]).getAllByText('Light 이상에서 이용 가능')).toHaveLength(1)
    expect(within(items[1]).getByText(/글 약/)).toBeInTheDocument()
    expect(within(items[2]).getAllByText(/글 약/)).toHaveLength(2)
    expect(within(items[2]).getByText('Pro 이상에서 이용 가능')).toBeInTheDocument()
    expect(within(items[2]).getByText('Max 이상에서 이용 가능')).toBeInTheDocument()
  })

  it('uses 30 assumed daily grants plus bonus, with no estimate when FX is unavailable', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: {
        plan: ProtoPlan.BASIC,
        estimatorCombos: [
          { combo: 'balanced', perPhotoMilli: 0, perThousandCharsMilli: 0, perPostBaseMilli: 1000 },
        ],
      },
    })
    const items = await cards()
    expect(screen.getByText(/30회 받는다고 가정하고 월 보너스를 더해/)).toBeInTheDocument()
    expect(
      screen.getByText(/예상 계산 환율 · test 2026-09-30 · 기준 1,400원\/달러, 적용 1,400원\/달러/),
    ).toBeInTheDocument()
    expect(within(items[2]).getByText('글 약 1860편 제작 가능')).toBeInTheDocument()
  })

  it('keeps missing prices and capabilities unavailable, and blog and clip inputs independent', async () => {
    const user = userEvent.setup()
    renderAppAt('/plans', {
      user: USER,
      plans: { plan: ProtoPlan.BASIC, estimatorCombos: [{ combo: 'balanced', clipRates: null }] },
    })
    const items = await cards()
    expect(within(items[2]).getAllByText('모델·가격 정보 준비 중')).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: '조건 바꾸기' }))
    fireEvent.change(screen.getByRole('slider', { name: '사진' }), { target: { value: '12' } })
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('tab', { name: '클립 기준' }))
    expect(screen.getByText('원본 영상은 개당 60초로 가정해요.')).toBeInTheDocument()
    expect(within(items[2]).getAllByText('클립 계산 가능한 모델 미지정')).toHaveLength(2)
    await user.click(screen.getByRole('button', { name: '조건 바꾸기' }))
    expect(screen.getByRole('slider', { name: '원본 영상 개수' })).toHaveAttribute('min', '1')
    expect(screen.getByRole('slider', { name: '원본 영상 개수' })).toHaveAttribute('max', '20')
    expect(screen.getByRole('slider', { name: '완성할 클립 길이' })).toHaveAttribute('min', '15')
    expect(screen.getByRole('slider', { name: '완성할 클립 길이' })).toHaveAttribute('max', '60')
    fireEvent.change(screen.getByRole('slider', { name: '원본 영상 개수' }), {
      target: { value: '4' },
    })
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('tab', { name: '블로그 글 기준' }))
    expect(screen.getByText('1000자 · 사진 12장 · 영상 0개 기준')).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '클립 기준' }))
    await waitFor(() =>
      expect(screen.getByText('원본 영상 4개 → 완성 클립 30초')).toBeInTheDocument(),
    )
  })

  it('does not invent counts without the current FX rate', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: { plan: ProtoPlan.BASIC, fxRate: null, fxUnavailable: true },
    })
    const items = await cards()
    expect(screen.getByText(/현재 환율 정보가 없어/)).toBeInTheDocument()
    expect(within(items[2]).queryByText(/글 약/)).not.toBeInTheDocument()
  })
})
