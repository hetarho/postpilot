import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  BillingService,
  GetMyBillingResponseSchema,
  ProtoPlan,
  ProtoTerm,
  QuotePriceResponseSchema,
  RegisterPaymentMethodResponseSchema,
  RemovePaymentMethodResponseSchema,
  SubscribeResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeBillingOptions {
  calls?: string[]
  populated?: boolean
  paymentMethod?: boolean
  subscription?: boolean
  subscribeRequests?: Array<{ plan: ProtoPlan; term: ProtoTerm }>
  subscribeFailure?: 'CHARGE_FAILED' | 'PAYMENT_METHOD_REQUIRED' | 'EMAIL_VERIFICATION_REQUIRED'
  registrationRequests?: Array<{ authKey: string; customerKey: string }>
  registerFailure?: 'EMAIL_VERIFICATION_REQUIRED' | 'CUSTOMER_KEY_MISMATCH' | 'BILLING_UNAVAILABLE'
  removeFailure?: 'SUBSCRIPTION_NEEDS_METHOD' | 'BILLING_UNAVAILABLE'
}

export function registerBillingService(router: ConnectRouter, options: FakeBillingOptions = {}) {
  const {
    calls,
    populated,
    paymentMethod,
    subscription,
    subscribeRequests,
    subscribeFailure,
    registrationRequests,
    registerFailure,
    removeFailure,
  } = options
  let subscribed = Boolean(populated || subscription)
  let hasPaymentMethod = Boolean(populated || paymentMethod)
  router.rpc(BillingService.method.getMyBilling, () => {
    calls?.push('GetMyBilling')
    return create(GetMyBillingResponseSchema, {
      customerKey: 'customer-key',
      subscription: subscribed
        ? {
            plan: ProtoPlan.PRO,
            term: ProtoTerm.MONTHLY,
            anchorAt: '2026-09-07T16:00:00Z',
            termStart: '2026-09-07T16:00:00Z',
            termEnd: '2026-10-08T00:00:00Z',
            nextGrantAt: '2026-10-08T00:00:00Z',
            autoRenew: true,
            status: 'active',
          }
        : undefined,
      paymentMethod: hasPaymentMethod
        ? { cardLabel: '11 1234', registeredAt: '2026-09-08T00:00:00Z' }
        : undefined,
      history: subscribed
        ? [
            {
              id: 2n,
              kind: 'tier_change',
              plan: ProtoPlan.PRO,
              term: ProtoTerm.MONTHLY,
              createdAt: '2026-09-08T00:00:01Z',
            },
            {
              id: 1n,
              kind: 'charge',
              plan: ProtoPlan.PRO,
              term: ProtoTerm.MONTHLY,
              usdCents: 500,
              krwPerUsdE4: 14000000n,
              rateDate: '2026-09-07',
              krw: 7000n,
              createdAt: '2026-09-08T00:00:00Z',
            },
          ]
        : [],
      purchases: populated
        ? [{ id: 'purchase-1', lotId: 'lot-1', credits: 100, usdCents: 100, krw: 1400n }]
        : [],
    })
  })
  router.rpc(BillingService.method.quotePrice, (request) => {
    const monthly =
      request.plan === ProtoPlan.BASIC ? 200 : request.plan === ProtoPlan.MAX ? 1000 : 500
    const usdCents = request.term === ProtoTerm.ANNUAL ? monthly * 10 : monthly
    return create(QuotePriceResponseSchema, {
      usdCents,
      krw: BigInt(usdCents * 14),
      krwPerUsdE4: 14000000n,
      rateDate: '2026-09-07',
    })
  })
  router.rpc(BillingService.method.registerPaymentMethod, (request) => {
    calls?.push('RegisterPaymentMethod')
    registrationRequests?.push({ authKey: request.authKey, customerKey: request.customerKey })
    if (registerFailure) {
      throw connectAppError(
        registerFailure,
        registerFailure === 'CUSTOMER_KEY_MISMATCH'
          ? Code.InvalidArgument
          : Code.FailedPrecondition,
      )
    }
    hasPaymentMethod = true
    return create(RegisterPaymentMethodResponseSchema, {
      paymentMethod: { cardLabel: '11 1234', registeredAt: '2026-09-08T00:00:00Z' },
      bonusGranted: true,
    })
  })
  router.rpc(BillingService.method.removePaymentMethod, () => {
    calls?.push('RemovePaymentMethod')
    if (removeFailure) throw connectAppError(removeFailure, Code.FailedPrecondition)
    return create(RemovePaymentMethodResponseSchema, {})
  })
  router.rpc(BillingService.method.subscribe, (request) => {
    calls?.push('Subscribe')
    subscribeRequests?.push({ plan: request.plan, term: request.term })
    if (subscribeFailure) throw connectAppError(subscribeFailure, Code.FailedPrecondition)
    subscribed = true
    return create(SubscribeResponseSchema, {
      subscription: {
        plan: request.plan,
        term: request.term,
        anchorAt: '2026-09-08T00:00:00Z',
        termStart: '2026-09-08T00:00:00Z',
        termEnd:
          request.term === ProtoTerm.ANNUAL ? '2027-09-08T00:00:00Z' : '2026-10-08T00:00:00Z',
        nextGrantAt: '2026-10-08T00:00:00Z',
        autoRenew: true,
        status: 'active',
      },
    })
  })
}
