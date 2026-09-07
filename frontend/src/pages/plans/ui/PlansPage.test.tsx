import { beforeEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ProtoPlan } from '@/shared/api'
import { PLAN_ESTIMATE_STORAGE_KEY } from '@/shared/config'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice' }

// The estimator keeps the reader's own case in localStorage, so one case's sliders would
// otherwise become the next case's defaults.
beforeEach(() => {
  localStorage.removeItem(PLAN_ESTIMATE_STORAGE_KEY)
})

async function rungs() {
  // The ladder arrives with GetMyPlan, so every case waits for the list rather than for the
  // title the page paints immediately.
  return within(await screen.findByRole('list')).getAllByRole('listitem')
}

describe('the plan comparison', () => {
  it('lists every rung with what its grant buys, side by side upward', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.BASIC },
      plans: {
        plan: ProtoPlan.BASIC,
        balance: { credits: 137, unlimited: false, monthlyGrant: 220 },
      },
    })

    expect(await screen.findByRole('heading', { name: '플랜', level: 1 })).toBeInTheDocument()
    const items = await rungs()
    expect(items).toHaveLength(4)

    // A phone stacks; the shape changes at `md:` and only reaches four columns on the desk.
    expect(screen.getByRole('list')).toHaveClass('grid', 'md:grid-cols-2', 'lg:grid-cols-4')

    // Every figure is the server's, including how many posts the grant covers (QUOTA-36).
    expect(within(items[1]).getByText('매달 220 크레딧')).toBeInTheDocument()
    expect(within(items[1]).getByText('월 $2')).toBeInTheDocument()

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

    // Every rung wears the promotional stroke; the recommended one wears it wider and casts a
    // shadow (THEME-37), and the badge above is what carries the meaning.
    const items = await rungs()
    const frames = items.map((item) => item.firstElementChild as HTMLElement)
    expect(frames.filter((frame) => frame.querySelector('[data-promo-stroke]'))).toHaveLength(4)
    const wide = frames.filter((frame) => frame.classList.contains('shadow-md'))
    expect(wide).toHaveLength(1)
    expect(wide[0]).toHaveClass('p-0.5')
    expect(within(wide[0]).getByRole('heading', { name: 'Pro' })).toBeInTheDocument()
    expect(within(wide[0]).getByText('가장 합리적')).toBeInTheDocument()
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
  })
})

describe('the promotional stroke', () => {
  // THEME-37 scopes the one gradient in this design language to a surface whose job is to be
  // chosen from. Another route rendering it would be the exception spreading.
  it('frames the ladder and its estimator, and nothing on another screen', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    await rungs()
    // Four rungs plus the estimator above them.
    expect(document.querySelectorAll('[data-promo-stroke]')).toHaveLength(5)
  })

  it('leaves other screens unframed', async () => {
    renderAppAt('/posts', { user: { ...USER, plan: ProtoPlan.FREE } })

    await screen.findByRole('heading', { name: '내 글' })
    expect(document.querySelectorAll('[data-promo-stroke]')).toHaveLength(0)
  })
})

describe('the plan estimator', () => {
  // QUOTA-41: the counts are derived from the three inputs and the selected combo's rates,
  // and the defaults are the shape most posts in this product have.
  it('counts posts for the default case on every rung', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    const items = await rungs()
    // 3800 + 5x723 + 1x3600 = 11,015 milli-credits a post, so 50 credits covers four.
    expect(within(items[0]).getByText('매달 약 4편')).toBeInTheDocument()
    expect(within(items[1]).getByText('매달 약 19편')).toBeInTheDocument()
    expect(within(items[2]).getByText('매달 약 52편')).toBeInTheDocument()
    expect(within(items[3]).getByText('매달 약 108편')).toBeInTheDocument()

    expect(screen.getByRole('slider', { name: '글자 수' })).toHaveValue('1000')
    expect(screen.getByRole('slider', { name: '사진' })).toHaveValue('5')
    expect(screen.getByRole('slider', { name: '영상' })).toHaveValue('0')
  })

  // The point of the control: the reader's own case, answered while they set it, with no
  // round trip (QUOTA-40).
  it('recomputes every count as the sliders move, asking nothing of the server', async () => {
    const calls: string[] = []
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      calls,
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })

    await rungs()
    const before = [...calls]

    // A shorter post with nothing attached: 3800 + 3600 = 7,400 milli a post.
    fireEvent.change(screen.getByRole('slider', { name: '글자 수' }), { target: { value: '600' } })
    fireEvent.change(screen.getByRole('slider', { name: '사진' }), { target: { value: '0' } })

    const items = await rungs()
    expect(within(items[1]).getByText('매달 약 29편')).toBeInTheDocument()
    expect(within(items[3]).getByText('매달 약 162편')).toBeInTheDocument()
    expect(calls).toEqual(before)
  })

  it('prices the chosen combo and remembers the case for the next visit', async () => {
    const user = userEvent.setup()
    const { unmount } = renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: {
        plan: ProtoPlan.FREE,
        balance: { credits: 50, unlimited: false, monthlyGrant: 50 },
        estimatorCombos: [
          { combo: 'balanced' },
          // A cheaper tier: half the per-post base and a tenth of the character rate.
          { combo: 'cheapest', perPostBaseMilli: 1900, perThousandCharsMilli: 360 },
        ],
      },
    })

    let items = await rungs()
    expect(within(items[1]).getByText('매달 약 19편')).toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: '최저가' }))
    items = await rungs()
    // 1900 + 5x723 + 360 = 5,875 milli a post.
    expect(within(items[1]).getByText('매달 약 37편')).toBeInTheDocument()

    fireEvent.change(screen.getByRole('slider', { name: '사진' }), { target: { value: '12' } })
    await waitFor(() => expect(screen.getByRole('slider', { name: '사진' })).toHaveValue('12'))
    unmount()

    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: { plan: ProtoPlan.FREE, balance: { credits: 50, unlimited: false, monthlyGrant: 50 } },
    })
    await rungs()
    expect(screen.getByRole('slider', { name: '사진' })).toHaveValue('12')
  })

  // A grant that cannot cover one post of the chosen shape is stated in words: "about 0
  // posts" is a figure nobody can act on.
  it('says a grant cannot cover one post rather than counting zero', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: {
        plan: ProtoPlan.FREE,
        balance: { credits: 50, unlimited: false, monthlyGrant: 50 },
        // A tier expensive enough that the free grant buys nothing of that shape.
        estimatorCombos: [{ combo: 'quality', perPostBaseMilli: 60_000 }],
      },
    })

    const items = await rungs()
    expect(within(items[0]).getByText('이 조건으로는 한 편도 어려워요')).toBeInTheDocument()
    expect(within(items[3]).getByText(/매달 약/)).toBeInTheDocument()
  })

  // An operator who has assigned nothing leaves the rungs saying what they grant and nothing
  // pretending to know what that buys.
  it('shows the grants with no count when no combo is assigned', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: {
        plan: ProtoPlan.FREE,
        balance: { credits: 50, unlimited: false, monthlyGrant: 50 },
        estimatorCombos: [],
      },
    })

    const items = await rungs()
    expect(within(items[1]).getByText('매달 220 크레딧')).toBeInTheDocument()
    expect(screen.queryByText(/매달 약/)).not.toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: '균형' })).not.toBeInTheDocument()
    expect(
      screen.getByText('아직 모델 조합이 지정되지 않아 편수를 계산할 수 없어요.'),
    ).toBeInTheDocument()
  })
})
