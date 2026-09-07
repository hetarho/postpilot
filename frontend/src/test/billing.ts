import { Code, createRouterTransport } from '@connectrpc/connect'
import { create } from '@bufbuild/protobuf'
import {
  BillingService,
  CancelScheduledChangeResponseSchema,
  CancelSubscriptionResponseSchema,
  ChangeSubscriptionResponseSchema,
  GetMyBillingResponseSchema,
  ProtoPlan,
  ProtoTerm,
  QuoteChangeResponseSchema,
  QuotePriceResponseSchema,
  RegisterPaymentMethodResponseSchema,
  RemovePaymentMethodResponseSchema,
  ResumeSubscriptionResponseSchema,
  SubscribeResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

function planRank(value: ProtoPlan) {
  if (value === ProtoPlan.BASIC) return 1
  if (value === ProtoPlan.PRO) return 2
  if (value === ProtoPlan.MAX) return 3
  return 0
}

export interface FakeBillingOptions {
  calls?: string[]
  populated?: boolean
  paymentMethod?: boolean
  subscription?: boolean
  subscriptionState?: {
    plan?: ProtoPlan
    term?: ProtoTerm
    autoRenew?: boolean
    scheduledPlan?: ProtoPlan
    scheduledTerm?: ProtoTerm
  }
  changeRequests?: Array<{ plan: ProtoPlan; term: ProtoTerm }>
  changeFailure?:
    | 'SUBSCRIPTION_REQUIRED'
    | 'NO_CHANGE'
    | 'NO_SCHEDULED_CHANGE'
    | 'CHANGE_UNSUPPORTED'
    | 'CHARGE_FAILED'
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
    subscriptionState,
    changeRequests,
    changeFailure,
    subscribeRequests,
    subscribeFailure,
    registrationRequests,
    registerFailure,
    removeFailure,
  } = options
  let subscribed = Boolean(populated || subscription)
  let hasPaymentMethod = Boolean(populated || paymentMethod)
  let currentPlan = subscriptionState?.plan ?? ProtoPlan.PRO
  let currentTerm = subscriptionState?.term ?? ProtoTerm.MONTHLY
  let autoRenew = subscriptionState?.autoRenew ?? true
  let scheduledPlan = subscriptionState?.scheduledPlan ?? ProtoPlan.UNSPECIFIED
  let scheduledTerm = subscriptionState?.scheduledTerm ?? ProtoTerm.UNSPECIFIED
  router.rpc(BillingService.method.getMyBilling, () => {
    calls?.push('GetMyBilling')
    return create(GetMyBillingResponseSchema, {
      customerKey: 'customer-key',
      subscription: subscribed
        ? {
            plan: currentPlan,
            term: currentTerm,
            anchorAt: '2026-09-07T16:00:00Z',
            termStart: '2026-09-07T16:00:00Z',
            termEnd: '2026-10-08T00:00:00Z',
            nextGrantAt: '2026-10-08T00:00:00Z',
            autoRenew,
            scheduledPlan,
            scheduledTerm,
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
    calls?.push('QuotePrice')
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
  router.rpc(BillingService.method.quoteChange, (request) => {
    calls?.push('QuoteChange')
    const monthly =
      request.plan === ProtoPlan.BASIC ? 200 : request.plan === ProtoPlan.MAX ? 1000 : 500
    const currentMonthly =
      currentPlan === ProtoPlan.BASIC ? 200 : currentPlan === ProtoPlan.MAX ? 1000 : 500
    const appliedNow =
      planRank(request.plan) > planRank(currentPlan) && request.term === currentTerm
    const usdCents = appliedNow ? Math.max(0, monthly - currentMonthly) : monthly
    return create(QuoteChangeResponseSchema, {
      usdCents,
      krw: BigInt(usdCents * 14),
      krwPerUsdE4: 14000000n,
      rateDate: '2026-09-07',
      appliedNow,
      effectiveAt: appliedNow ? '2026-09-08T00:00:00Z' : '2026-10-08T00:00:00Z',
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
    currentPlan = request.plan
    currentTerm = request.term
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
  router.rpc(BillingService.method.changeSubscription, (request) => {
    calls?.push('ChangeSubscription')
    changeRequests?.push({ plan: request.plan, term: request.term })
    if (changeFailure) {
      throw connectAppError(
        changeFailure,
        changeFailure === 'CHANGE_UNSUPPORTED' ? Code.InvalidArgument : Code.FailedPrecondition,
      )
    }
    const appliedNow =
      planRank(request.plan) > planRank(currentPlan) && request.term === currentTerm
    if (appliedNow) {
      currentPlan = request.plan
      scheduledPlan = ProtoPlan.UNSPECIFIED
      scheduledTerm = ProtoTerm.UNSPECIFIED
    } else {
      scheduledPlan = request.plan
      scheduledTerm = request.term
      autoRenew = true
    }
    return create(ChangeSubscriptionResponseSchema, {
      appliedNow,
      subscription: {
        plan: currentPlan,
        term: currentTerm,
        anchorAt: '2026-09-07T16:00:00Z',
        termStart: '2026-09-07T16:00:00Z',
        termEnd: '2026-10-08T00:00:00Z',
        nextGrantAt: '2026-10-08T00:00:00Z',
        autoRenew,
        scheduledPlan,
        scheduledTerm,
        status: 'active',
      },
    })
  })
  router.rpc(BillingService.method.cancelScheduledChange, () => {
    calls?.push('CancelScheduledChange')
    if (changeFailure === 'NO_SCHEDULED_CHANGE') {
      throw connectAppError(changeFailure, Code.FailedPrecondition)
    }
    scheduledPlan = ProtoPlan.UNSPECIFIED
    scheduledTerm = ProtoTerm.UNSPECIFIED
    return create(CancelScheduledChangeResponseSchema, {
      subscription: { plan: currentPlan, term: currentTerm, autoRenew, status: 'active' },
    })
  })
  router.rpc(BillingService.method.cancelSubscription, () => {
    calls?.push('CancelSubscription')
    if (changeFailure === 'SUBSCRIPTION_REQUIRED' || changeFailure === 'NO_CHANGE') {
      throw connectAppError(changeFailure, Code.FailedPrecondition)
    }
    autoRenew = false
    scheduledPlan = ProtoPlan.UNSPECIFIED
    scheduledTerm = ProtoTerm.UNSPECIFIED
    return create(CancelSubscriptionResponseSchema, {
      subscription: {
        plan: currentPlan,
        term: currentTerm,
        termEnd: '2026-10-08T00:00:00Z',
        autoRenew,
        status: 'active',
      },
    })
  })
  router.rpc(BillingService.method.resumeSubscription, () => {
    calls?.push('ResumeSubscription')
    if (changeFailure === 'SUBSCRIPTION_REQUIRED' || changeFailure === 'NO_CHANGE') {
      throw connectAppError(changeFailure, Code.FailedPrecondition)
    }
    autoRenew = true
    return create(ResumeSubscriptionResponseSchema, {
      subscription: {
        plan: currentPlan,
        term: currentTerm,
        termEnd: '2026-10-08T00:00:00Z',
        autoRenew,
        status: 'active',
      },
    })
  })
}
