import { describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

const USER = { id: 'alice' }

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
    expect(within(items[1]).getByText('매달 약 6편')).toBeInTheDocument()
    expect(within(items[1]).getByText('월 $2')).toBeInTheDocument()
    expect(within(items[3]).getByText('매달 약 37편')).toBeInTheDocument()
    expect(
      screen.getByText(
        '편수는 사진 10장·표준 길이 글 하나를 넉넉하게 잡아 계산한 값이라, 실제로는 더 많이 쓸 수 있어요.',
      ),
    ).toBeInTheDocument()

    // The current rung is named and offers nothing to press; the rest carry the seam a
    // checkout will attach to, disabled with the operator path stated beside it.
    expect(within(items[1]).getByText('지금 쓰는 플랜')).toBeInTheDocument()
    expect(within(items[1]).queryByRole('button')).not.toBeInTheDocument()
    expect(within(items[0]).getByRole('button', { name: '이 플랜 선택하기' })).toBeDisabled()
    expect(
      screen.getByText('결제는 아직 준비 중이에요. 플랜 변경은 운영자에게 문의해 주세요.'),
    ).toBeInTheDocument()
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

    const proCard = marked[0].closest('div.rounded-lg') as HTMLElement
    expect(proCard).toHaveClass('border', 'border-stroke-accent', 'shadow-md')
    expect(within(proCard).getByRole('heading', { name: 'Pro' })).toBeInTheDocument()

    // No other rung carries a stroke: the accent marks a choice, it does not decorate.
    const stroked = (await rungs()).filter((item) =>
      item.firstElementChild?.classList.contains('border-stroke-accent'),
    )
    expect(stroked).toHaveLength(1)
  })

  it('states a grant too small for one post rather than promising zero', async () => {
    renderAppAt('/plans', {
      user: { ...USER, plan: ProtoPlan.FREE },
      plans: {
        plan: ProtoPlan.FREE,
        balance: { credits: 50, unlimited: false, monthlyGrant: 50 },
        offers: [{ plan: ProtoPlan.FREE, monthlyCredits: 10, priceUsdCents: 0, estimatedPosts: 0 }],
      },
    })

    await rungs()
    expect(screen.getByText('한 편을 다 쓰기엔 모자라요')).toBeInTheDocument()
    expect(screen.queryByText(/약 0편/)).not.toBeInTheDocument()
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
