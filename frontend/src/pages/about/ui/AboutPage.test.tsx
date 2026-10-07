import { afterEach, describe, expect, it, vi } from 'vitest'
import { readFileSync } from 'node:fs'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
import { createFakeAuthTransport } from '@/test/session'
import { PUBLIC_LADDER } from '../model/ladder'

const CANONICAL = readFileSync('../backend/internal/plan/offers.go', 'utf8')
const TIERS = [
  {
    plan: 'free',
    monthlyKrw: 0,
    annualKrw: 0,
    dailyCredits: 0,
    monthlyBonus: 0,
    monthlyServerExports: 0,
    modelCeiling: 'none',
  },
  {
    plan: 'light',
    monthlyKrw: 1900,
    annualKrw: 19000,
    dailyCredits: 15,
    monthlyBonus: 290,
    monthlyServerExports: 0,
    modelCeiling: 'value',
  },
  {
    plan: 'basic',
    monthlyKrw: 4900,
    annualKrw: 49000,
    dailyCredits: 45,
    monthlyBonus: 510,
    monthlyServerExports: 0,
    modelCeiling: 'balanced',
  },
  {
    plan: 'pro',
    monthlyKrw: 9900,
    annualKrw: 99000,
    dailyCredits: 85,
    monthlyBonus: 1070,
    monthlyServerExports: 0,
    modelCeiling: 'premium',
  },
  {
    plan: 'max',
    monthlyKrw: 29900,
    annualKrw: 299000,
    dailyCredits: 235,
    monthlyBonus: 3170,
    monthlyServerExports: 60,
    modelCeiling: 'top',
  },
] as const

afterEach(() => initializeI18n('ko'))

describe('public offer mirror', () => {
  it('matches the backend commercial table including the annual ten-month charge', () => {
    expect(
      PUBLIC_LADDER.map((offer) => ({
        plan: offer.plan,
        monthlyKrw: offer.monthlyKrw,
        annualKrw: offer.annualKrw,
        dailyCredits: offer.dailyCredits,
        monthlyBonus: offer.monthlyBonus,
        monthlyServerExports: offer.monthlyServerExports,
        modelCeiling: offer.modelCeiling,
      })),
    ).toEqual(TIERS)
    for (const tier of TIERS.slice(1)) {
      expect(CANONICAL).toContain(
        `{${tier.monthlyKrw}, ${tier.annualKrw}, ${tier.dailyCredits}, ${tier.monthlyBonus}, "${tier.modelCeiling}", ${tier.monthlyServerExports}}`,
      )
      expect(tier.annualKrw).toBe(tier.monthlyKrw * 10)
    }
    expect(PUBLIC_LADDER.filter((offer) => offer.recommended).map((offer) => offer.plan)).toEqual([
      'pro',
    ])
  })
})

describe.each(['ko', 'en'] as const)('About page in %s', (locale) => {
  it('presents AI assistance, phrase-origin review, owner edits and manual publication', async () => {
    initializeI18n(locale)
    renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    expect(
      screen.getByText(
        locale === 'ko'
          ? 'AI는 속도를, 글의 주도권은 나에게'
          : 'AI helps you move faster. You stay in control of your writing.',
      ),
    ).toBeVisible()
    const hero = screen.getByRole('region', {
      name:
        locale === 'ko'
          ? '사진과 메모를 내 말투의 블로그 초안으로'
          : 'Photos and rough notes into a blog draft in your own voice',
    })
    expect(hero).toHaveTextContent(
      locale === 'ko' ? 'AI는 표현과 구성을 돕고' : 'AI helps with phrasing and structure',
    )

    const flow = within(
      screen.getByRole('region', { name: locale === 'ko' ? '어떻게 쓰나요' : 'How it works' }),
    )
    const reviewAndPublish = flow.getAllByRole('listitem')[3]!
    for (const origin of locale === 'ko'
      ? ['직접 입력 기반', '사진에서 추론', 'AI가 보탠 내용']
      : ['based on your input', 'inferred from photos', 'added by AI']) {
      expect(reviewAndPublish).toHaveTextContent(origin)
    }
    expect(reviewAndPublish).toHaveTextContent(
      locale === 'ko'
        ? '필요한 문장을 직접 고치거나 AI 수정을 요청하고'
        : 'Edit the sentences yourself',
    )
    expect(reviewAndPublish).toHaveTextContent(
      locale === 'ko'
        ? '플랫폼 형식으로 복사해 목적지 서비스에 직접 게시합니다'
        : 'copy the finalized post in your platform’s format and publish manually',
    )
    const outputs = screen.getByRole('region', {
      name: locale === 'ko' ? '결과물은 어디로 가나요' : 'Where the result goes',
    })
    expect(outputs).toHaveTextContent(
      locale === 'ko'
        ? '출처 표시와 기술 정보는 복사한 글에 섞이지 않습니다'
        : 'Origin labels and technical details stay out of the copied writing',
    )
    expect(screen.getByRole('main')).not.toHaveTextContent(
      /\d+\s*(?:배 더 빠르|times faster|x faster)|사실 보장|정확도 보장|AI 없는 글|검색 순위 향상|자동 (?:발행|게시)|guaranteed truth|AI-free writing|search-rank gain|automatically publish/i,
    )
  })

  it.each([
    ['anonymous', undefined],
    ['signed in', { id: 'marketing-owner' }],
  ] as const)(
    'mounts for a %s visitor without any private or model RPC',
    async (_session, user) => {
      initializeI18n(locale)
      const calls: string[] = []
      const transport = createFakeAuthTransport({ user, calls })
      // Observe the real transport entry points as well as registered fake procedures: even an
      // unregistered private/model/inspection request must fail this network-free public contract.
      const unary = vi.spyOn(transport, 'unary')
      const stream = vi.spyOn(transport, 'stream')
      const { router } = renderAppAt('/about', { transport })
      await screen.findByRole('heading', { level: 1 })
      await waitFor(() => expect(router.state.isLoading).toBe(false))

      expect(router.state.location.pathname).toBe('/about')
      expect(calls).toEqual([])
      expect(unary).not.toHaveBeenCalled()
      expect(stream).not.toHaveBeenCalled()
    },
  )

  it('keeps the public explanation, signup path and ordered flow', async () => {
    initializeI18n(locale)
    const { container } = renderAppAt('/about?redirect=%2Fposts%2Fwelcome')
    const headings = await screen.findAllByRole('heading', { level: 1 })
    expect(headings).toHaveLength(1)
    const flow = within(
      screen.getByRole('region', { name: locale === 'ko' ? '어떻게 쓰나요' : 'How it works' }),
    )
    expect(flow.getAllByRole('listitem')).toHaveLength(4)
    expect(flow.getAllByRole('listitem')[0].closest('ol')).not.toBeNull()
    expect(
      screen.getByRole('link', { name: locale === 'ko' ? '시작하기' : 'Get started' }),
    ).toHaveAttribute('href', '/signup')
    expect(
      screen.getByRole('link', { name: locale === 'ko' ? '로그인' : 'Log in' }),
    ).toHaveAttribute('href', '/login?redirect=%2Fposts%2Fwelcome')
    expect(container.querySelectorAll('.bg-button-cta-bg')).toHaveLength(1)
    expect(container.querySelector('img, picture, video, audio, iframe, embed, object')).toBeNull()
  })

  it('renders five public cards with monthly and annual KRW prices without a plan request or checkout', async () => {
    initializeI18n(locale)
    const calls: string[] = []
    const { container } = renderAppAt('/about', { calls })
    const region = within(
      await screen.findByRole('region', { name: locale === 'ko' ? '요금제' : 'Plans' }),
    )
    // Each card holds its own benefit list, so the rungs are the ladder's direct children.
    const cards = Array.from(region.getAllByRole('list')[0].children) as HTMLElement[]
    expect(cards).toHaveLength(5)
    for (const [index, tier] of TIERS.entries()) {
      const card = within(cards[index])
      expect(
        card.getByRole('heading', {
          level: 3,
          name: tier.plan[0].toUpperCase() + tier.plan.slice(1),
        }),
      ).toBeInTheDocument()
      if (!tier.monthlyKrw) {
        // Free names what it gives, never a zero-valued benefit (QUOTA-28).
        expect(card.getByText(locale === 'ko' ? '무료 모델' : 'Free models')).toBeInTheDocument()
        expect(card.queryByText(/\b0\b/)).not.toBeInTheDocument()
        continue
      }
      expect(card.getAllByText(new RegExp(tier.dailyCredits.toString())).length).toBeGreaterThan(0)
      if (tier.monthlyServerExports > 0) {
        expect(card.getByText(/(서버 내보내기|server exports)/)).toHaveTextContent('60')
      } else {
        expect(card.queryByText(/(서버 내보내기|server exports)/)).not.toBeInTheDocument()
      }
      expect(
        card.getByText(new RegExp(tier.monthlyKrw.toLocaleString('en-US'))),
      ).toBeInTheDocument()
      expect(card.getByText(new RegExp(tier.annualKrw.toLocaleString('en-US')))).toBeInTheDocument()
    }
    expect(calls).toEqual([])
    expect(region.queryByRole('button')).not.toBeInTheDocument()
    expect(region.queryByRole('link')).not.toBeInTheDocument()
    expect(container.querySelector('form, input, textarea')).toBeNull()
    expect(region.getAllByRole('list')[0]).toHaveClass('md:grid-cols-2', 'xl:grid-cols-5')
    expect(
      region.getAllByText(
        locale === 'ko'
          ? '클립 원본·완성본 최대 60초'
          : '60-second cap for source and finished clips',
      ),
    ).toHaveLength(1)
  })
})

it('keeps the public header keyboard order and lets cramped rows wrap', async () => {
  initializeI18n('ko')
  const user = userEvent.setup()
  const { container } = renderAppAt('/about')
  await screen.findByRole('heading', { level: 1 })
  const header = container.querySelector('header')
  expect(header).toHaveClass('flex-wrap', 'gap-y-2')
  await user.tab()
  expect(screen.getByRole('link', { name: '시작하기' })).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('button', { name: '테마' })).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('button', { name: '언어' })).toHaveFocus()
  await user.tab()
  expect(screen.getByRole('link', { name: '로그인' })).toHaveFocus()
})
