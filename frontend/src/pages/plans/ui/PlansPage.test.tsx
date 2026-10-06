import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { PostCreditsBasis, ProtoPlan, ProtoTerm } from '@/shared/api'
import { CLIP_ESTIMATE_STORAGE_KEY } from '../config'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice', plan: ProtoPlan.BASIC }
const MASTER = { id: 'root', plan: ProtoPlan.MASTER }
const ALL_LEVELS = [
  { combo: 'value' },
  { combo: 'balanced' },
  { combo: 'premium' },
  { combo: 'top' },
]

beforeEach(() => {
  localStorage.removeItem(CLIP_ESTIMATE_STORAGE_KEY)
})
afterEach(() => initializeI18n('ko'))

/** The five cards, in ladder order. Each card holds its own benefit list, so the rungs are the
 *  ladder's direct children rather than every list item on the page. */
async function cards() {
  const region = await screen.findByRole('region', { name: '플랜' })
  return Array.from(within(region).getAllByRole('list')[0].children) as HTMLElement[]
}

async function comparisonRows(name = '한 달 예상 제작량') {
  const region = await screen.findByRole('region', { name })
  return Array.from(within(region).getByRole('list').children) as HTMLElement[]
}

/** A card's one button, whether it is a link into billing or a button. */
function cardControls(card: HTMLElement) {
  return [...within(card).queryAllByRole('link'), ...within(card).queryAllByRole('button')]
}

describe('the plan cards', () => {
  it('holds tier, price, one button and current benefits, with shared copy outside', async () => {
    renderAppAt('/plans', { user: USER, plans: { plan: ProtoPlan.BASIC } })
    const items = await cards()
    expect(items).toHaveLength(5)
    expect(
      within(await screen.findByRole('region', { name: '플랜' })).getAllByRole('list')[0],
    ).toHaveClass('md:grid-cols-2', 'xl:grid-cols-5')
    for (const [index, name, monthly, daily, bonus, exports, level] of [
      [1, 'Light', 1900, 15, 290, 0, '가성비'],
      [2, 'Basic', 4900, 45, 510, 0, '밸런스'],
      [3, 'Pro', 9900, 85, 1070, 0, '고급'],
      [4, 'Max', 29900, 235, 3170, 60, '최고'],
    ] as const) {
      const card = within(items[index])
      expect(card.getByRole('heading', { name })).toBeInTheDocument()
      expect(card.getByText(`${monthly.toLocaleString('en-US')}원`)).toBeInTheDocument()
      expect(
        within(card.getByRole('list'))
          .getAllByRole('listitem')
          .map((item) => item.textContent),
      ).toEqual([
        `매일 ${daily} 크레딧 지급`,
        `매월 보너스 ${bonus} 크레딧`,
        `무료 모델 + ${level} 등급까지`,
        ...(exports > 0 ? [`서버 내보내기 월 ${exports}회`] : []),
      ])
      expect(cardControls(items[index])).toHaveLength(1)
    }
    // Free names what it gives and nothing it lacks: no zero-valued benefit.
    const free = within(items[0])
    expect(free.getByText('무료')).toBeInTheDocument()
    expect(
      within(free.getByRole('list'))
        .getAllByRole('listitem')
        .map((item) => item.textContent),
    ).toEqual(['무료 모델', '무료 모델은 제공사 제한에 따라 사용'])
    expect(free.queryByText(/0 크레딧|월 0회/)).not.toBeInTheDocument()
    for (const card of items) {
      expect(within(card).queryByText(/최대 60초|글 약|편|크레딧 · /)).not.toBeInTheDocument()
    }
    expect(screen.getAllByText('클립 원본·완성본 최대 60초')).toHaveLength(1)
    expect(screen.getAllByText('가장 합리적')).toHaveLength(1)
    expect(within(items[3]).getByText('가장 합리적')).toBeInTheDocument()
    expect(within(items[2]).getByText('지금 쓰는 플랜')).toBeInTheDocument()
    expect(within(items[2]).queryByText('가장 합리적')).not.toBeInTheDocument()
  })

  it('shows the annual charge with one monthly line and states the saving once, outside the cards', async () => {
    const user = userEvent.setup()
    renderAppAt('/plans', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE },
    })
    const items = await cards()
    expect(screen.queryByText('12개월을 10개월 요금으로 결제해요')).not.toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '연간 · 약 16.7% 절약' }))
    for (const [index, monthly, annual] of [
      [1, 1900, 19000],
      [2, 4900, 49000],
      [3, 9900, 99000],
      [4, 29900, 299000],
    ] as const) {
      const card = within(items[index])
      expect(card.getByText(`${annual.toLocaleString('en-US')}원`)).toBeInTheDocument()
      expect(
        card.getByText(`월 약 ${Math.round(annual / 12).toLocaleString('en-US')}원꼴`),
      ).toBeInTheDocument()
      expect(card.queryByText(/10개월|16.7%/)).not.toBeInTheDocument()
      expect(card.queryByText(`${monthly.toLocaleString('en-US')}원`)).not.toBeInTheDocument()
    }
    expect(screen.getAllByText('12개월을 10개월 요금으로 결제해요')).toHaveLength(1)
    expect(within(items[0]).getByText('무료')).toBeInTheDocument()
    expect(within(items[3]).getByRole('link', { name: '구독하기' })).toHaveAttribute(
      'href',
      '/billing/checkout?tier=pro&term=annual',
    )
  })
})

describe('each card’s one button', () => {
  it('gives a free account Subscribe on every paid card and a disabled current free card', async () => {
    const { container } = renderAppAt('/plans', {
      user: { id: 'alice', plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 0, unlimited: false } },
    })
    const items = await cards()
    expect(within(items[0]).getByRole('button', { name: '이용 중' })).toBeDisabled()
    expect(within(items[0]).getByText('지금 쓰는 플랜')).toBeInTheDocument()
    for (const [index, tier] of [
      [1, 'light'],
      [2, 'basic'],
      [3, 'pro'],
      [4, 'max'],
    ] as const) {
      expect(within(items[index]).getByRole('link', { name: '구독하기' })).toHaveAttribute(
        'href',
        `/billing/checkout?tier=${tier}&term=monthly`,
      )
    }
    // Pro's Subscribe is the view's one filled CTA (THEME-18).
    expect(container.querySelectorAll('.bg-button-cta-bg')).toHaveLength(1)
    expect(within(items[3]).getByRole('link', { name: '구독하기' })).toHaveClass('bg-button-cta-bg')
    expect(await screen.findByText('크레딧이 부족해요')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '크레딧 구매' })).toHaveAttribute('href', '/billing')
  })

  it('keeps upgrade, scheduled change, management and cancellation in their billing paths', async () => {
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
    for (const card of items) expect(cardControls(card)).toHaveLength(1)
    expect(
      within(items[0]).getByRole('link', { name: '구독 해지는 결제 관리에서' }),
    ).toHaveAttribute('href', '/billing')
    expect(within(items[3]).getByRole('link', { name: '구독 관리' })).toHaveAttribute(
      'href',
      '/billing',
    )
    expect(within(items[4]).getByRole('link', { name: '업그레이드' })).toHaveAttribute(
      'href',
      '/billing/checkout?tier=max&term=monthly',
    )
    expect(within(items[1]).getByRole('button', { name: '다음 결제일부터' })).toHaveClass('w-full')
    await user.click(within(items[2]).getByRole('button', { name: '다음 결제일부터' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: '변경 예약' }))
    await waitFor(() =>
      expect(changeRequests).toEqual([{ plan: ProtoPlan.BASIC, term: ProtoTerm.MONTHLY }]),
    )
  })

  // BILL-20, QUOTA-68: the operator sees a customer's first-purchase buttons, every one disabled,
  // and no notice about being the operator.
  it('shows the operator the customer buttons, all disabled, with no operator notice', async () => {
    const { container } = renderAppAt('/plans', {
      user: MASTER,
      plans: { plan: ProtoPlan.MASTER, balance: { unlimited: true } },
    })
    const items = await cards()
    expect(within(items[0]).getByRole('button', { name: '무료 플랜' })).toBeDisabled()
    for (const card of items.slice(1)) {
      expect(within(card).getByRole('button', { name: '구독하기' })).toBeDisabled()
      expect(within(card).queryByRole('link')).not.toBeInTheDocument()
    }
    expect(screen.queryByText(/운영자 계정|결제 없이 모든 기능/)).not.toBeInTheDocument()
    expect(container.querySelectorAll('.bg-button-cta-bg')).toHaveLength(0)
    expect(screen.queryByText(/환율|원\/USD|원\/달러/)).not.toBeInTheDocument()
  })
})

describe('the comparison below the cards', () => {
  it('lists every tier’s posts and AI clips on its own highest level', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: { plan: ProtoPlan.BASIC, estimatorCombos: ALL_LEVELS },
    })
    const rows = await comparisonRows()
    expect(rows).toHaveLength(5)
    expect(screen.getByText(/30회 받는다고 가정하고 월 보너스를 더해/)).toBeInTheDocument()
    const free = within(rows[0])
    expect(free.getByRole('heading', { name: 'Free' })).toBeInTheDocument()
    expect(free.getByText('무료 모델은 제공사 제한에 따라 사용')).toBeInTheDocument()
    expect(free.queryByText(/약/)).not.toBeInTheDocument()
    // 15 credits a post and a 3 × 60 s → 30 s clip at 48.25 credits, over 30 daily grants + bonus.
    for (const [index, level, posts, clips] of [
      [1, '가성비', 49, 15],
      [2, '밸런스', 124, 38],
      [3, '고급', 241, 75],
      [4, '최고', 681, 211],
    ] as const) {
      const row = within(rows[index])
      expect(row.getByText(`${level} 모델 기준`)).toBeInTheDocument()
      expect(await row.findByText(`약 ${posts}편`)).toBeInTheDocument()
      expect(await row.findByText(`약 ${clips}편`)).toBeInTheDocument()
      expect(row.getByText('글 1개당 약 15크레딧 · 최근 사용량 기준')).toBeInTheDocument()
    }
    expect(screen.queryByRole('tab', { name: '블로그 글 기준' })).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '클립 기준' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '조건 바꾸기' })).not.toBeInTheDocument()
    expect(screen.queryByText('등급별 AI 모델 이용')).not.toBeInTheDocument()
  })

  // QUOTA-64: a level whose pair has too little recent usage prices a post from the catalog
  // and says so.
  it('labels an estimated per-post figure as one', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: {
        plan: ProtoPlan.BASIC,
        estimatorCombos: [
          { combo: 'balanced', postCredits: 20, postCreditsBasis: PostCreditsBasis.ESTIMATE },
        ],
      },
    })
    const rows = await comparisonRows()
    expect(within(rows[2]).getByText('글 1개당 약 20크레딧 · 예상')).toBeInTheDocument()
    expect(await within(rows[2]).findByText('약 93편')).toBeInTheDocument()
  })

  // QUOTA-56: a tier whose level has no assigned pair, or no clip rate, says so instead of
  // borrowing a lower level's count.
  it('explains missing pairs and clip rates without borrowing another level', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: {
        plan: ProtoPlan.BASIC,
        estimatorCombos: [{ combo: 'value' }, { combo: 'balanced', clipRates: null }],
      },
    })
    const rows = await comparisonRows()
    expect(await within(rows[2]).findByText('약 124편')).toBeInTheDocument()
    expect(within(rows[2]).getByText('클립 계산 가능한 모델 미지정')).toBeInTheDocument()
    for (const row of [rows[3], rows[4]]) {
      expect(within(row).getAllByText('모델·가격 정보 준비 중')).toHaveLength(2)
      expect(within(row).queryByText(/약 \d/)).not.toBeInTheDocument()
    }
  })

  it('does not invent counts while paid estimates are unavailable', async () => {
    renderAppAt('/plans', {
      user: USER,
      plans: { plan: ProtoPlan.BASIC, fxUnavailable: true },
    })
    const rows = await comparisonRows()
    expect(screen.getByText('지금은 예상 편수를 계산할 수 없어요.')).toBeInTheDocument()
    expect(screen.queryByText(/환율/)).not.toBeInTheDocument()
    expect(within(rows[2]).getAllByText('지금은 계산할 수 없어요')).toHaveLength(2)
    expect(within(rows[2]).queryByText(/약 \d/)).not.toBeInTheDocument()
  })

  // QUOTA-41, QUOTA-57: the clip conditions live inline in the comparison, start at three
  // originals and 30 seconds, stay in the browser and recount without a request.
  it('edits the clip conditions inline and recounts without a request', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/plans', {
      user: USER,
      calls,
      plans: { plan: ProtoPlan.BASIC, estimatorCombos: ALL_LEVELS },
    })
    const rows = await comparisonRows()
    const toggle = screen.getByRole('button', {
      name: /클립 조건 · 원본 영상 3개 → 완성 클립 30초/,
    })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('slider')).not.toBeInTheDocument()
    await user.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('원본 영상은 개당 60초로 가정해요.')).toBeInTheDocument()
    const sources = screen.getByRole('slider', { name: '원본 영상 개수' })
    expect(sources).toHaveAttribute('min', '1')
    expect(sources).toHaveAttribute('max', '20')
    const seconds = screen.getByRole('slider', { name: '완성할 클립 길이' })
    expect(seconds).toHaveAttribute('min', '15')
    expect(seconds).toHaveAttribute('max', '60')
    const before = calls.length
    fireEvent.change(sources, { target: { value: '4' } })
    expect(
      await screen.findByRole('button', { name: /원본 영상 4개 → 완성 클립 30초/ }),
    ).toBeInTheDocument()
    // 11.2 + 4 × 8.75 + 30 × 0.36 = 57 credits a clip, so Basic's 1860 credits cover 32.
    expect(await within(rows[2]).findByText('약 32편')).toBeInTheDocument()
    expect(calls.length).toBe(before)
    expect(JSON.parse(localStorage.getItem(CLIP_ESTIMATE_STORAGE_KEY) ?? '{}')).toEqual({
      sources: 4,
      seconds: 30,
    })
  })

  it('reads in English', async () => {
    initializeI18n('en')
    renderAppAt('/plans', {
      user: { id: 'alice', plan: ProtoPlan.PRO },
      plans: { plan: ProtoPlan.PRO, estimatorCombos: ALL_LEVELS },
      billing: {
        subscription: true,
        subscriptionState: { plan: ProtoPlan.PRO, term: ProtoTerm.MONTHLY },
      },
    })
    const region = await screen.findByRole('region', { name: 'Plans' })
    const items = Array.from(within(region).getAllByRole('list')[0].children) as HTMLElement[]
    expect(within(items[3]).getByRole('link', { name: 'Manage subscription' })).toHaveAttribute(
      'href',
      '/billing',
    )
    const rows = await comparisonRows('Estimated monthly output')
    expect(within(rows[3]).getByText('On Premium models')).toBeInTheDocument()
    expect(await within(rows[3]).findByText('About 241')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Clip conditions/ })).toBeInTheDocument()
  })
})
