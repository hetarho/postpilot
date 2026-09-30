import { afterEach, describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'
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
    monthlyServerExports: 2,
    modelCeiling: 'value',
  },
  {
    plan: 'basic',
    monthlyKrw: 4900,
    annualKrw: 49000,
    dailyCredits: 45,
    monthlyBonus: 510,
    monthlyServerExports: 6,
    modelCeiling: 'balanced',
  },
  {
    plan: 'pro',
    monthlyKrw: 9900,
    annualKrw: 99000,
    dailyCredits: 85,
    monthlyBonus: 1070,
    monthlyServerExports: 15,
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
    const cards = region.getAllByRole('listitem')
    expect(cards).toHaveLength(5)
    for (const [index, tier] of TIERS.entries()) {
      const card = within(cards[index])
      expect(
        card.getByRole('heading', {
          level: 3,
          name: tier.plan[0].toUpperCase() + tier.plan.slice(1),
        }),
      ).toBeInTheDocument()
      expect(card.getAllByText(new RegExp(tier.dailyCredits.toString())).length).toBeGreaterThan(0)
      expect(
        card.getAllByText(new RegExp(tier.monthlyServerExports.toString())).length,
      ).toBeGreaterThan(0)
      if (tier.monthlyKrw) {
        expect(
          card.getByText(new RegExp(tier.monthlyKrw.toLocaleString('en-US'))),
        ).toBeInTheDocument()
        expect(
          card.getByText(new RegExp(tier.annualKrw.toLocaleString('en-US'))),
        ).toBeInTheDocument()
      }
    }
    expect(calls).toEqual([])
    expect(region.queryByRole('button')).not.toBeInTheDocument()
    expect(region.queryByRole('link')).not.toBeInTheDocument()
    expect(container.querySelector('form, input, textarea')).toBeNull()
    expect(region.getByRole('list')).toHaveClass('md:grid-cols-2', 'xl:grid-cols-5')
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
