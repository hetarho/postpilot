import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { GetMyPlanResponseSchema, PostCreditsBasis, ProtoPlan } from '@/shared/api'
import { toMyPlan } from './plan-mappers'

describe('toMyPlan offers', () => {
  it('carries the recommended mark the server sent', () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.BASIC,
        offers: [
          {
            plan: ProtoPlan.LIGHT,
            monthlyKrw: 1900,
            annualKrw: 19000,
            dailyCredits: 15,
            monthlyBonus: 290,
            modelCeiling: 'value',
            monthlyServerExports: 2,
          },
          {
            plan: ProtoPlan.PRO,
            monthlyKrw: 9900,
            annualKrw: 99000,
            dailyCredits: 85,
            monthlyBonus: 1070,
            modelCeiling: 'premium',
            monthlyServerExports: 15,
            recommended: true,
          },
        ],
        creditPacks: [{ id: 'pack-1000', priceKrw: 3000, credits: 1000 }],
      }),
    )

    expect(myPlan?.offers).toEqual([
      {
        plan: 'light',
        monthlyKrw: 1900,
        annualKrw: 19000,
        dailyCredits: 15,
        monthlyBonus: 290,
        modelCeiling: 'value',
        monthlyServerExports: 2,
        recommended: false,
      },
      {
        plan: 'pro',
        monthlyKrw: 9900,
        annualKrw: 99000,
        dailyCredits: 85,
        monthlyBonus: 1070,
        modelCeiling: 'premium',
        monthlyServerExports: 15,
        recommended: true,
      },
    ])
    expect(myPlan?.creditPacks).toEqual([{ id: 'pack-1000', priceKrw: 3000, credits: 1000 }])
  })
})

it('keeps server export use, reservations, remaining and renewal separate from credits', () => {
  const mapped = toMyPlan(
    create(GetMyPlanResponseSchema, {
      plan: ProtoPlan.BASIC,
      balance: { credits: 28, renewsAt: '2026-10-01T00:00:00Z' },
      serverExportWindow: {
        coverageId: 'paid:alice',
        startsAt: '2026-09-01T00:00:00Z',
        endsAt: '2026-10-01T00:00:00Z',
        allowance: 6,
        used: 2,
        reserved: 1,
        remaining: 3,
      },
    }),
  )
  expect(mapped?.balance.credits).toBe(28)
  expect(mapped?.serverExportWindow).toMatchObject({
    used: 2,
    reserved: 1,
    remaining: 3,
    allowance: 6,
    endsAt: '2026-10-01T00:00:00Z',
  })
})

describe('toMyPlan lots', () => {
  // QUOTA-58: a voucher lot keeps its own kind; only a kind the client has never heard of
  // falls back to a bonus.
  it('keeps each known kind and reads an unknown one as a bonus', () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.FREE,
        balance: {
          credits: 40,
          lots: [
            { kind: 'voucher', granted: 10, remaining: 10, expiresAt: '2026-10-01T00:00:00Z' },
            { kind: 'monthly', granted: 10, remaining: 10, expiresAt: '2026-10-02T00:00:00Z' },
            { kind: 'bonus', granted: 10, remaining: 10 },
            { kind: 'purchased', granted: 10, remaining: 10 },
            { kind: 'gift', granted: 1, remaining: 1 },
          ],
        },
      }),
    )

    expect(myPlan?.balance.lots.map((lot) => lot.kind)).toEqual([
      'voucher',
      'monthly',
      'bonus',
      'purchased',
      'bonus',
    ])
  })
})

describe('toMyPlan estimator combos', () => {
  // QUOTA-64: a level carries one post's credits and where the figure came from; a figure with
  // no basis this build can name is no figure at all.
  it("carries each level's per-post figure with its basis", () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.FREE,
        estimatorCombos: [
          {
            combo: 'balanced',
            observeLabel: 'vendor/eyes',
            writeLabel: 'vendor/pen',
            postCredits: 38,
            postCreditsBasis: PostCreditsBasis.RECENT_USAGE,
          },
          { combo: 'premium', postCredits: 90, postCreditsBasis: PostCreditsBasis.ESTIMATE },
          { combo: 'top', postCredits: 12, postCreditsBasis: PostCreditsBasis.UNSPECIFIED },
        ],
      }),
    )

    expect(myPlan?.estimatorCombos).toEqual([
      {
        combo: 'balanced',
        observeLabel: 'vendor/eyes',
        writeLabel: 'vendor/pen',
        postCredits: { credits: 38, basis: 'recent' },
      },
      {
        combo: 'premium',
        observeLabel: '',
        writeLabel: '',
        postCredits: { credits: 90, basis: 'estimate' },
      },
      { combo: 'top', observeLabel: '', writeLabel: '' },
    ])
  })

  // The four are a closed set. A tier this build cannot name is nothing a screen can offer
  // as a choice, so it is dropped rather than rendered as an unknown option.
  it('drops a combo it cannot name and tolerates none at all', () => {
    const unknown = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.FREE,
        estimatorCombos: [{ combo: 'legacy', postCredits: 10 }],
      }),
    )
    expect(unknown?.estimatorCombos).toEqual([])

    const none = toMyPlan(create(GetMyPlanResponseSchema, { plan: ProtoPlan.FREE }))
    expect(none?.estimatorCombos).toEqual([])
  })
})

it('preserves optional clip prices and the source assumption without inventing a free quote', () => {
  const clipRates = { perSourceMilli: 1234, perOutputSecondMilli: 56, perClipBaseMilli: 7890 }
  const mapped = toMyPlan(
    create(GetMyPlanResponseSchema, {
      clipSourceSeconds: 60,
      estimatorCombos: [{ combo: 'value', clipRates }, { combo: 'balanced' }],
    }),
  )
  expect(mapped?.clipSourceSeconds).toBe(60)
  expect(mapped?.estimatorCombos[0].clipRates).toEqual(clipRates)
  expect(mapped?.estimatorCombos[1].clipRates).toBeUndefined()
})
