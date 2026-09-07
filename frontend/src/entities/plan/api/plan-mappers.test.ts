import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { GetMyPlanResponseSchema, ProtoPlan } from '@/shared/api'
import { toMyPlan } from './plan-mappers'

describe('toMyPlan offers', () => {
  it('carries the estimate and the recommended mark the server sent', () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.BASIC,
        offers: [
          { plan: ProtoPlan.BASIC, monthlyCredits: 220, priceUsdCents: 200, estimatedPosts: 6 },
          {
            plan: ProtoPlan.PRO,
            monthlyCredits: 575,
            priceUsdCents: 500,
            estimatedPosts: 17,
            recommended: true,
          },
        ],
      }),
    )

    expect(myPlan?.offers).toEqual([
      {
        plan: 'basic',
        monthlyCredits: 220,
        priceUsdCents: 200,
        estimatedPosts: 6,
        recommended: false,
      },
      {
        plan: 'pro',
        monthlyCredits: 575,
        priceUsdCents: 500,
        estimatedPosts: 17,
        recommended: true,
      },
    ])
  })

  // A server that says nothing about either must not produce a highlighted rung: emphasis
  // the ladder did not ask for would be the client inventing a recommendation.
  it('reads an offer that omits both as no estimate and not marked', () => {
    const myPlan = toMyPlan(
      create(GetMyPlanResponseSchema, {
        plan: ProtoPlan.FREE,
        offers: [{ plan: ProtoPlan.FREE, monthlyCredits: 50, priceUsdCents: 0 }],
      }),
    )

    expect(myPlan?.offers).toEqual([
      {
        plan: 'free',
        monthlyCredits: 50,
        priceUsdCents: 0,
        estimatedPosts: 0,
        recommended: false,
      },
    ])
  })
})
