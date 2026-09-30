import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import {
  GetMyBillingResponseSchema,
  ProtoPlan,
  ProtoTerm,
  QuotePriceResponseSchema,
  QuotePurchaseResponseSchema,
} from '@/shared/api'
import { toMyBilling, toPurchaseQuote, toQuote } from './billing-mappers'

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
          },
        ],
        purchases: [
          {
            id: 'p1',
            packId: 'pack-1000',
            credits: 100,
            krw: 1400n,
            refundable: true,
          },
        ],
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
      history: [{ id: 7n, krw: 70000n }],
      purchases: [{ id: 'p1', packId: 'pack-1000', credits: 100, krw: 1400n, refundable: true }],
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
          quoteId: 'q-fixed',
          krw: 6963n,
        }),
      ),
    ).toEqual({
      id: 'q-fixed',
      krw: 6963n,
    })
  })

  it('maps fixed KRW offers and pack IDs without inventing a checkout FX rate', () => {
    expect(toQuote(create(QuotePriceResponseSchema, { krw: 19000n }))).toMatchObject({
      krw: 19000n,
    })
    expect(
      toPurchaseQuote(
        create(QuotePurchaseResponseSchema, { packId: 'pack-3000', credits: 3000, krw: 9000n }),
      ),
    ).toMatchObject({ packId: 'pack-3000', credits: 3000, krw: 9000n })
  })
})
