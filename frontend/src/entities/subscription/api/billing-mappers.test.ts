import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  GetMyBillingResponseSchema,
  ProtoPlan,
  ProtoTerm,
  QuotePriceResponseSchema,
} from '@/shared/api'
import { toMyBilling, toQuote } from './billing-mappers'

describe('billing mappers', () => {
  it('maps presence, enums, bigint money, and lists without deriving figures', () => {
    const mapped = toMyBilling(
      create(GetMyBillingResponseSchema, {
        customerKey: 'customer-key',
        subscription: {
          plan: ProtoPlan.PRO,
          term: ProtoTerm.ANNUAL,
          scheduledPlan: ProtoPlan.BASIC,
          scheduledTerm: ProtoTerm.MONTHLY,
          autoRenew: true,
        },
        paymentMethod: { cardLabel: '11 1234', registeredAt: '2026-09-08T00:00:00Z' },
        history: [
          {
            id: 7n,
            kind: 'charge',
            plan: ProtoPlan.PRO,
            term: ProtoTerm.ANNUAL,
            krw: 70000n,
            krwPerUsdE4: 14000000n,
          },
        ],
        purchases: [{ id: 'p1', credits: 100, usdCents: 100, krw: 1400n, refundable: true }],
      }),
    )
    expect(mapped).toMatchObject({
      customerKey: 'customer-key',
      subscription: {
        plan: 'pro',
        term: 'annual',
        scheduledPlan: 'basic',
        scheduledTerm: 'monthly',
        autoRenew: true,
      },
      paymentMethod: { cardLabel: '11 1234' },
      history: [{ id: 7n, krw: 70000n, krwPerUsdE4: 14000000n }],
      purchases: [{ id: 'p1', credits: 100, krw: 1400n, refundable: true }],
    })
  })

  it('keeps an absent subscription and payment method absent and maps a quote exactly', () => {
    expect(toMyBilling(create(GetMyBillingResponseSchema, {}))).toMatchObject({
      subscription: undefined,
      paymentMethod: undefined,
      history: [],
      purchases: [],
    })
    expect(
      toQuote(
        create(QuotePriceResponseSchema, {
          usdCents: 500,
          krw: 6963n,
          krwPerUsdE4: 13925000n,
          rateDate: '2026-09-07',
        }),
      ),
    ).toEqual({ usdCents: 500, krw: 6963n, ratePerUsdE4: 13925000n, rateDate: '2026-09-07' })
  })
})
