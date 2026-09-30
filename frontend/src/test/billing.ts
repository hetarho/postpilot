import { Code, createRouterTransport } from '@connectrpc/connect'
import { create, type MessageInitShape } from '@bufbuild/protobuf'
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
  QuotePurchaseResponseSchema,
  PurchaseCreditsResponseSchema,
  RequestRefundResponseSchema,
  ListMyRefundsResponseSchema,
  ListRefundReviewsResponseSchema,
  ReviewRefundResponseSchema,
  ReconcileRefundResponseSchema,
  RegisterPaymentMethodResponseSchema,
  RemovePaymentMethodResponseSchema,
  ResumeSubscriptionResponseSchema,
  SubscribeResponseSchema,
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

function planRank(value: ProtoPlan) {
  if (value === ProtoPlan.LIGHT) return 1
  if (value === ProtoPlan.BASIC) return 2
  if (value === ProtoPlan.PRO) return 3
  if (value === ProtoPlan.MAX) return 4
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
    anchorAt?: string
    termEnd?: string
    nextGrantAt?: string
  }
  changeRequests?: Array<{ plan: ProtoPlan; term: ProtoTerm }>
  changeFailure?:
    | 'SUBSCRIPTION_REQUIRED'
    | 'NO_CHANGE'
    | 'NO_SCHEDULED_CHANGE'
    | 'CHANGE_UNSUPPORTED'
    | 'CHARGE_FAILED'
  subscribeRequests?: Array<{ plan: ProtoPlan; term: ProtoTerm }>
  subscribePending?: boolean
  subscribeFailure?: 'CHARGE_FAILED' | 'PAYMENT_METHOD_REQUIRED' | 'EMAIL_VERIFICATION_REQUIRED'
  registrationRequests?: Array<{ authKey: string; customerKey: string }>
  registerFailure?: 'EMAIL_VERIFICATION_REQUIRED' | 'CUSTOMER_KEY_MISMATCH' | 'BILLING_UNAVAILABLE'
  removeFailure?: 'SUBSCRIPTION_NEEDS_METHOD' | 'BILLING_UNAVAILABLE'
  purchaseRequests?: Array<number | string>
  refundRequests?: string[]
  refundReviewCalls?: Array<{ requestId: string; outcome: string; amount: bigint }>
  initialRefunds?: Array<{
    id: string
    userId: string
    reason: string
    status: string
    requestedAt: string
    payment: { orderId: string; kind: string; chargedKrw: bigint; chargedAt: string }
  }>
  purchaseFailure?: 'CHARGE_FAILED' | 'PAYMENT_METHOD_REQUIRED' | 'PURCHASE_TOO_SMALL'
  refundFailure?: 'REFUND_FAILED'
  /** Rows listed above the populated history, newest first, e.g. a refund. */
  extraHistory?: NonNullable<MessageInitShape<typeof GetMyBillingResponseSchema>['history']>
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
    subscribePending,
    registrationRequests,
    registerFailure,
    removeFailure,
    purchaseRequests,
    refundRequests,
    refundReviewCalls,
    initialRefunds = [],
    purchaseFailure,
    refundFailure,
    extraHistory = [],
  } = options
  let subscribed = Boolean(populated || subscription)
  let hasPaymentMethod = Boolean(populated || paymentMethod)
  let currentPlan = subscriptionState?.plan ?? ProtoPlan.PRO
  let currentTerm = subscriptionState?.term ?? ProtoTerm.MONTHLY
  let autoRenew = subscriptionState?.autoRenew ?? true
  let scheduledPlan = subscriptionState?.scheduledPlan ?? ProtoPlan.UNSPECIFIED
  let scheduledTerm = subscriptionState?.scheduledTerm ?? ProtoTerm.UNSPECIFIED
  let purchases = populated
    ? [
        {
          id: 'purchase-1',
          refundOrderId: 'purchase-1',
          credits: 100,
          krw: 1400n,
          chargedAt: '2026-09-08T00:00:00Z',
          refundedAt: '',
          refundable: true,
        },
      ]
    : []
  let refunds = [...initialRefunds]
  router.rpc(BillingService.method.getMyBilling, () => {
    calls?.push('GetMyBilling')
    return create(GetMyBillingResponseSchema, {
      customerKey: 'customer-key',
      subscription: subscribed
        ? {
            plan: currentPlan,
            term: currentTerm,
            anchorAt: subscriptionState?.anchorAt ?? '2026-09-07T16:00:00Z',
            termStart: '2026-09-07T16:00:00Z',
            termEnd: subscriptionState?.termEnd ?? '2026-10-08T00:00:00Z',
            nextGrantAt: subscriptionState?.nextGrantAt ?? '2026-10-08T00:00:00Z',
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
            ...extraHistory,
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
              krw: 7000n,
              orderId: 'sub-1',
              createdAt: '2026-09-08T00:00:00Z',
            },
          ]
        : [],
      purchases,
    })
  })
  router.rpc(BillingService.method.quotePrice, (request) => {
    calls?.push('QuotePrice')
    const monthly =
      request.plan === ProtoPlan.LIGHT
        ? 1900
        : request.plan === ProtoPlan.BASIC
          ? 4900
          : request.plan === ProtoPlan.MAX
            ? 29900
            : 9900
    return create(QuotePriceResponseSchema, {
      krw: BigInt(request.term === ProtoTerm.ANNUAL ? monthly * 10 : monthly),
    })
  })
  router.rpc(BillingService.method.quoteChange, (request) => {
    calls?.push('QuoteChange')
    const monthly =
      request.plan === ProtoPlan.LIGHT
        ? 1900
        : request.plan === ProtoPlan.BASIC
          ? 4900
          : request.plan === ProtoPlan.MAX
            ? 29900
            : 9900
    const currentMonthly =
      currentPlan === ProtoPlan.LIGHT
        ? 1900
        : currentPlan === ProtoPlan.BASIC
          ? 4900
          : currentPlan === ProtoPlan.MAX
            ? 29900
            : 9900
    const appliedNow =
      planRank(request.plan) > planRank(currentPlan) && request.term === currentTerm
    const krw = appliedNow ? Math.max(0, monthly - currentMonthly) : monthly
    return create(QuoteChangeResponseSchema, {
      krw: BigInt(krw),
      appliedNow,
      effectiveAt: appliedNow ? '2026-09-08T00:00:00Z' : '2026-10-08T00:00:00Z',
    })
  })
  const packs = [
    { id: 'pack-1000', credits: 1000, krw: 3000n },
    { id: 'pack-3000', credits: 3000, krw: 9000n },
    { id: 'pack-10000', credits: 10000, krw: 30000n },
  ]
  router.rpc(BillingService.method.quotePurchase, (request) => {
    calls?.push('QuotePurchase')
    const pack = packs.find((item) => item.id === request.packId) ?? packs[0]!
    return create(QuotePurchaseResponseSchema, {
      packId: pack.id,
      credits: pack.credits,
      krw: pack.krw,
    })
  })
  router.rpc(BillingService.method.purchaseCredits, (request) => {
    calls?.push('PurchaseCredits')
    purchaseRequests?.push(request.packId)
    if (purchaseFailure) throw connectAppError(purchaseFailure, Code.FailedPrecondition)
    const pack = packs.find((item) => item.id === request.packId) ?? packs[0]!
    const purchase = {
      id: `purchase-${purchases.length + 1}`,
      refundOrderId: `purchase-${purchases.length + 1}`,
      packId: pack.id,
      credits: pack.credits,
      krw: pack.krw,
      chargedAt: '2026-09-08T00:00:01Z',
      refundedAt: '',
      refundable: true,
    }
    purchases = [purchase, ...purchases]
    return create(PurchaseCreditsResponseSchema, { purchase })
  })
  router.rpc(BillingService.method.requestRefund, (request) => {
    calls?.push('RequestRefund')
    refundRequests?.push(request.orderId)
    if (refundFailure) throw connectAppError(refundFailure, Code.FailedPrecondition)
    const refund = {
      id: `refund-${refunds.length + 1}`,
      userId: 'alice',
      reason: request.reason,
      status: 'requested',
      requestedAt: '2026-09-08T00:00:02Z',
      payment: {
        orderId: request.orderId,
        kind: 'subscribe',
        chargedKrw: 7000n,
        chargedAt: '2026-09-08T00:00:00Z',
      },
    }
    refunds = [refund, ...refunds]
    return create(RequestRefundResponseSchema, { refund })
  })
  router.rpc(BillingService.method.listMyRefunds, () =>
    create(ListMyRefundsResponseSchema, { refunds }),
  )
  router.rpc(BillingService.method.listRefundReviews, () =>
    create(ListRefundReviewsResponseSchema, { refunds }),
  )
  router.rpc(BillingService.method.reviewRefund, (request) => {
    refundReviewCalls?.push({
      requestId: request.requestId,
      outcome: request.outcome,
      amount: request.reviewedAmountKrw,
    })
    refunds = refunds.map((refund) =>
      refund.id === request.requestId
        ? { ...refund, status: request.outcome === 'reject' ? 'rejected' : 'processing' }
        : refund,
    )
    return create(ReviewRefundResponseSchema, {
      refund: refunds.find((refund) => refund.id === request.requestId),
    })
  })
  router.rpc(BillingService.method.reconcileRefund, (request) => {
    refunds = refunds.map((refund) =>
      refund.id === request.requestId ? { ...refund, status: 'completed' } : refund,
    )
    return create(ReconcileRefundResponseSchema, {
      refund: refunds.find((refund) => refund.id === request.requestId),
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
      bonusGranted: false,
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
        status: subscribePending ? 'pending' : 'active',
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
