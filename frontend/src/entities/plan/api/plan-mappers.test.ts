import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { GetMyPlanResponseSchema, ProtoPlan } from '@/shared/api'
import { toMyPlan } from './plan-mappers'

describe('toMyPlan offers', () => {
  it('carries the recommended mark the server sent', () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.BASIC,
        offers: [
          { plan: ProtoPlan.BASIC, monthlyCredits: 220, priceUsdCents: 200 },
          { plan: ProtoPlan.PRO, monthlyCredits: 575, priceUsdCents: 500, recommended: true },
        ],
      }),
    )

    expect(myPlan?.offers).toEqual([
      { plan: 'basic', monthlyCredits: 220, priceUsdCents: 200, recommended: false },
      { plan: 'pro', monthlyCredits: 575, priceUsdCents: 500, recommended: true },
    ])
  })
})

describe('toMyPlan estimator combos', () => {
  it('carries every rate a client multiplies', () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.FREE,
        estimatorCombos: [
          {
            combo: 'balanced',
            observeLabel: 'vendor/eyes',
            writeLabel: 'vendor/pen',
            perPhotoMilli: 723,
            perVideoMilli: 1100,
            perThousandCharsMilli: 3600,
            perPostBaseMilli: 3800,
          },
        ],
      }),
    )

    expect(myPlan?.estimatorCombos).toEqual([
      {
        combo: 'balanced',
        observeLabel: 'vendor/eyes',
        writeLabel: 'vendor/pen',
        perPhotoMilli: 723,
        perVideoMilli: 1100,
        perThousandCharsMilli: 3600,
        perPostBaseMilli: 3800,
      },
    ])
  })

  // The four are a closed set. A tier this build cannot name is nothing a screen can offer
  // as a choice, so it is dropped rather than rendered as an unknown option.
  it('drops a combo it cannot name and tolerates none at all', () => {
    const unknown = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.FREE,
        estimatorCombos: [{ combo: 'legacy', perPostBaseMilli: 10 }],
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
