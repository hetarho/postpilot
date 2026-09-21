import { beforeEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan, ProtoTerm } from '@/shared/api'
import { CLIP_ESTIMATE_STORAGE_KEY, PLAN_ESTIMATE_STORAGE_KEY } from '../config'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice' }

// The estimator keeps the reader's own case in localStorage, so one case's sliders would
// otherwise become the next case's defaults.
beforeEach(() => {
  localStorage.removeItem(PLAN_ESTIMATE_STORAGE_KEY)
  localStorage.removeItem(CLIP_ESTIMATE_STORAGE_KEY)
})

async function rungs() {
  // The ladder arrives with GetMyPlan, so every case waits for the list rather than for the
  // title the page paints immediately.
  return within(await screen.findByRole('list')).getAllByRole('listitem')
}

describe('the plan comparison', () => {
  it('routes an active subscriber to upgrade, scheduled downgrade, and Billing cancellation', async () => {
    const user = userEvent.setup()
    const changeRequests: Array<{ plan: ProtoPlan; term: ProtoTerm }> = []
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.PRO },
      plans: { plan: ProtoPlan.PRO, balance: { credits: 500, unlimited: false } },
      billing: {
        subscription: true,
        subscriptionState: { plan: ProtoPlan.PRO, term: ProtoTerm.MONTHLY },
        changeRequests,
      },
    })

    const items = await rungs()
    expect(
      within(items[0]).getByRole('link', { name: '구독 해지는 결제 관리에서' }),
    ).toHaveAttribute('href', '/billing')
    expect(within(items[3]).getByRole('link', { name: '업그레이드' })).toHaveAttribute(
      'href',
      '/billing/checkout?tier=max',
    )
    await user.click(within(items[1]).getByRole('button', { name: '다음 결제일부터' }))
    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveTextContent('지금은 결제되지 않습니다.')
    await user.click(within(dialog).getByRole('button', { name: '변경 예약' }))
    await waitFor(() =>
      expect(changeRequests).toEqual([{ plan: ProtoPlan.BASIC, term: ProtoTerm.MONTHLY }]),
    )
  })

  it('lists every rung with what its grant buys, side by side upward', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.BASIC },
      plans: {
        plan: ProtoPlan.BASIC,
        balance: { credits: 137, unlimited: false, monthlyGrant: 330 },
      },
    })

    expect(
      await screen.findByRole('heading', { name: '기록은 더 많이.가능성은 더 넓게.', level: 1 }),
    ).toBeInTheDocument()
    const items = await rungs()
    expect(items).toHaveLength(4)

    // A phone stacks; the shape changes at `md:` and only reaches four columns on the desk.
    expect(screen.getByRole('list')).toHaveClass('grid', 'md:grid-cols-2', 'lg:grid-cols-4')

    // Every figure is the server's, including how many posts the grant covers (QUOTA-36).
    expect(within(items[1]).getByText('매달 330 크레딧')).toBeInTheDocument()
    // The price is the card's one hero figure, with the period beside it rather than inside it.
    expect(within(items[1]).getByText('$3')).toHaveClass('text-3xl')
    expect(within(items[1]).getByText('/ 월')).toBeInTheDocument()
    expect(within(items[0]).getByText('무료')).toHaveClass('text-3xl')
    expect(within(items[0]).queryByText('/ 월')).not.toBeInTheDocument()

    // The current and free rungs offer nothing to press; another paid rung enters checkout.
    expect(within(items[1]).getByText('지금 쓰는 플랜')).toBeInTheDocument()
    expect(within(items[1]).queryByRole('link')).not.toBeInTheDocument()
    expect(within(items[0]).queryByRole('link')).not.toBeInTheDocument()
    expect(within(items[2]).getByRole('link', { name: '구독하기' })).toHaveAttribute(
      'href',
      '/billing/checkout?tier=pro',
    )
  })

  // THEME-37 allows exactly one marked option per promotional surface, and it hangs off the
  // server's flag so a second one cannot appear by editing this page.
  it('marks exactly one rung as the recommendation', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    await rungs()
    const marked = screen.getAllByText('가장 합리적')
    expect(marked).toHaveLength(1)

    // Every rung wears the promotional stroke; the recommended one wears it wider, casts a
    // shadow and glows (THEME-37), and the badge above is what carries the meaning.
    const items = await rungs()
    const frames = items.map((item) => item.firstElementChild as HTMLElement)
    expect(frames.filter((frame) => frame.querySelector('[data-promo-stroke]'))).toHaveLength(4)
    const wide = frames.filter((frame) => frame.classList.contains('shadow-md'))
    expect(wide).toHaveLength(1)
    expect(wide[0].querySelector('[data-promo-stroke]')).toHaveClass('p-0.5')
    expect(within(wide[0]).getByRole('heading', { name: 'Pro' })).toBeInTheDocument()
    expect(within(wide[0]).getByText('가장 합리적')).toBeInTheDocument()
    // Exactly one halo, and it belongs to the marked rung; the desk lifts that rung a step.
    expect(document.querySelectorAll('[data-promo-glow]')).toHaveLength(1)
    expect(wide[0].querySelector('[data-promo-glow]')).not.toBeNull()
    expect(wide[0]).toHaveClass('lg:scale-105')
    // Its subscribe action is the view's one filled CTA; every other rung stays secondary.
    expect(within(wide[0]).getByRole('link', { name: '구독하기' })).toHaveClass('bg-button-cta-bg')
    expect(within(items[3]).getByRole('link', { name: '구독하기' })).toHaveClass(
      'bg-button-secondary-bg',
    )
    expect(document.querySelectorAll('.bg-button-cta-bg')).toHaveLength(1)
  })

  // The ladder arrives as one gesture: each rung rises a beat after the one before it, once, on
  // mount — sliders moving afterwards recompute the figures without replaying it.
  it('staggers the rungs into place on arrival', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    const items = await rungs()
    const frames = items.map((item) => item.firstElementChild as HTMLElement)
    for (const frame of frames) expect(frame).toHaveClass('animate-rise')
    expect(frames.map((frame) => frame.style.animationDelay)).toEqual([
      '0ms',
      '60ms',
      '120ms',
      '180ms',
    ])
  })

  it('offers no subscription actions to an operator account', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.MASTER },
      plans: { plan: ProtoPlan.MASTER, balance: { unlimited: true } },
    })

    await rungs()
    expect(screen.queryByRole('link', { name: '구독하기' })).not.toBeInTheDocument()
  })

  // QUOTA-29: an exhausted balance blocks AI work and nothing else, which is the one thing
  // someone who arrived here from a refusal needs told.
  it('tells an exhausted account what still works', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 0, unlimited: false, monthlyGrant: 50 } },
    })

    // The loading line is a `status` too, so the notice is found by its text.
    const notice = await screen.findByText('크레딧이 부족해요')
    expect(notice.closest('[role="status"]')).toHaveTextContent(
      '글을 쓰고 고치고 내보내는 건 그대로 할 수 있어요.',
    )
    expect(screen.getByRole('link', { name: '크레딧 구매' })).toHaveAttribute('href', '/billing')
  })
})

describe('the promotional stroke', () => {
  // THEME-37 scopes the one gradient in this design language to a surface whose job is to be
  // chosen from. Another route rendering it would be the exception spreading.
  it('frames the four rungs without an inline condition editor', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    await rungs()
    // Four rungs share the full viewport aurora stage.
    expect(document.querySelectorAll('[data-promo-stroke]')).toHaveLength(4)
    expect(document.querySelectorAll('[data-promo-aurora]')).toHaveLength(1)
    // The hero figure is gradient ink, clipped to the glyphs.
    expect(screen.getByText('$10')).toHaveClass('bg-promo-text', 'bg-clip-text', 'text-transparent')
  })

  it('leaves other screens unframed', async () => {
    renderAppAt('/posts', { user: { ...USER, plan: ProtoPlan.FREE } })

    await screen.findByRole('heading', { name: '내 글' })
    expect(document.querySelectorAll('[data-promo-stroke]')).toHaveLength(0)
    expect(document.querySelectorAll('[data-promo-aurora]')).toHaveLength(0)
  })
})

describe('blog and clip conditions', () => {
  it('shows all four model levels at once with each assigned price on every plan', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: {
        estimatorCombos: ['value', 'balanced', 'premium', 'top'].map((combo, index) => ({
          combo,
          perPhotoMilli: 835 * (index + 1),
          perVideoMilli: 1399 * (index + 1),
          perThousandCharsMilli: 5400 * (index + 1),
          perPostBaseMilli: 4700 * (index + 1),
        })),
      },
    })
    const items = await rungs()
    for (const [index, grant] of [50, 330, 1150, 2400].entries()) {
      expect(
        within(items[index])
          .getAllByRole('term')
          .map((el) => el.textContent),
      ).toEqual([
        '가성비 모델 사용 시',
        '밸런스 모델 사용 시',
        '고급 모델 사용 시',
        '최고 모델 사용 시',
      ])
      expect(
        within(items[index])
          .getAllByRole('definition')
          .map((el) => el.textContent),
      ).toEqual(
        [1, 2, 3, 4].map((factor) => {
          const count = Math.floor((grant * 1000) / (14275 * factor))
          return count ? `글 약 ${count}편 제작 가능` : '이 조건으로 1편 미만'
        }),
      )
    }
    expect(screen.queryByRole('slider')).not.toBeInTheDocument()
    expect(screen.getByRole('tab', { name: '블로그 글 기준' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    expect(screen.getByRole('button', { name: '조건 바꾸기' }).parentElement).toHaveClass(
      'fixed',
      'right-4',
    )
  })

  it('edits only per-post conditions, recomputes every plan without RPCs and restores focus', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/plans', { user: USER, calls })
    await rungs()
    const opener = screen.getByRole('button', { name: '조건 바꾸기' })
    await user.click(opener)
    const sheet = await screen.findByRole('dialog', { name: '글 한 편의 조건' })
    expect(within(sheet).getAllByRole('slider')).toHaveLength(3)
    expect(within(sheet).queryByRole('tab')).not.toBeInTheDocument()
    expect(within(sheet).queryByText(/제작 가능/)).not.toBeInTheDocument()
    const before = [...calls]
    fireEvent.change(within(sheet).getByRole('slider', { name: '글자 수' }), {
      target: { value: '600' },
    })
    fireEvent.change(within(sheet).getByRole('slider', { name: '사진' }), {
      target: { value: '0' },
    })
    await user.keyboard('{Escape}')
    expect(opener).toHaveFocus()
    const items = await rungs()
    await waitFor(() =>
      expect(within(items[1]).getAllByText('글 약 32편 제작 가능')).toHaveLength(2),
    )
    await waitFor(() =>
      expect(within(items[3]).getAllByText('글 약 237편 제작 가능')).toHaveLength(2),
    )
    expect(calls).toEqual(before)
    await user.click(opener)
    expect(screen.getByRole('slider', { name: '사진' })).toHaveValue('0')
  })

  it('uses finished duration for clip pricing and persists both condition sets independently', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { unmount } = renderAppAt('/plans', { user: USER, calls })
    await rungs()
    await user.click(screen.getByRole('button', { name: '조건 바꾸기' }))
    fireEvent.change(screen.getByRole('slider', { name: '사진' }), { target: { value: '12' } })
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('tab', { name: '클립 기준' }))
    expect(screen.getByText('원본 영상 3개 → 완성 클립 30초')).toBeInTheDocument()
    expect(screen.getByText('원본 영상은 개당 60초로 가정해요.')).toBeInTheDocument()
    expect(within((await rungs())[1]).getAllByText('클립 약 6편 제작 가능')).toHaveLength(2)
    await user.click(screen.getByRole('button', { name: '조건 바꾸기' }))
    const sheet = await screen.findByRole('dialog', { name: '클립 한 편의 조건' })
    expect(within(sheet).getAllByRole('slider')).toHaveLength(2)
    const before = [...calls]
    fireEvent.change(within(sheet).getByRole('slider', { name: '원본 영상 개수' }), {
      target: { value: '4' },
    })
    fireEvent.change(within(sheet).getByRole('slider', { name: '완성할 클립 길이' }), {
      target: { value: '45' },
    })
    await user.click(within(sheet).getByRole('button', { name: '이 조건으로 비교하기' }))
    // 11200 + 4*8750 + 45*360 = 62400 milli-credits.
    await waitFor(() =>
      expect(
        within(screen.getAllByRole('listitem')[1]).getAllByText('클립 약 5편 제작 가능'),
      ).toHaveLength(2),
    )
    expect(calls).toEqual(before)
    await user.click(screen.getByRole('tab', { name: '블로그 글 기준' }))
    expect(screen.getByText('1000자 · 사진 12장 · 영상 0개 기준')).toBeInTheDocument()
    unmount()
    renderAppAt('/plans', { user: USER })
    await rungs()
    expect(screen.getByText('1000자 · 사진 12장 · 영상 0개 기준')).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '클립 기준' }))
    expect(screen.getByText('원본 영상 4개 → 완성 클립 45초')).toBeInTheDocument()
  })

  it('keeps unassigned and clip-incompatible levels visible without fabricating zero estimates', async () => {
    const user = userEvent.setup()
    renderAppAt('/plans', {
      user: USER,
      plans: { estimatorCombos: [{ combo: 'top', clipRates: null }] },
    })
    const items = await rungs()
    expect(within(items[1]).getAllByText('모델·가격 정보 준비 중')).toHaveLength(3)
    expect(within(items[1]).getByText('글 약 23편 제작 가능')).toBeInTheDocument()
    await user.click(screen.getByRole('tab', { name: '클립 기준' }))
    expect(within(items[1]).getAllByText('클립 계산 가능한 모델 미지정')).toHaveLength(4)
    expect(screen.queryByText(/클립 약/)).not.toBeInTheDocument()
  })

  it('keeps four unavailable rows when no models are assigned', async () => {
    renderAppAt('/plans', { user: USER, plans: { estimatorCombos: [] } })
    const items = await rungs()
    for (const item of items)
      expect(within(item).getAllByText('모델·가격 정보 준비 중')).toHaveLength(4)
    expect(screen.queryByText(/글 약/)).not.toBeInTheDocument()
  })

  it('states the top-up baseline and derives each paid bonus from its offer', async () => {
    renderAppAt('/plans', { user: { ...USER, plan: ProtoPlan.FREE } })
    const items = await rungs()
    expect(
      screen.getByText('일반 충전은 $1당 100크레딧. 구독하면 매달 더 받아요.'),
    ).toBeInTheDocument()
    expect(within(items[0]).queryByText(/크레딧 추가/)).not.toBeInTheDocument()
    for (const [index, credits, percent] of [
      [1, 30, 10],
      [2, 150, 15],
      [3, 400, 20],
    ]) {
      expect(within(items[index]).getByText(`매달 ${credits}크레딧 추가`)).toBeInTheDocument()
      expect(within(items[index]).getByText(`같은 금액 충전보다 +${percent}%`)).toBeInTheDocument()
    }
    expect(document.querySelector('[data-promo-stage="viewport"]')).toHaveClass('fixed', 'inset-0')
  })
})
