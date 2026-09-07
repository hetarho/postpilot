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
} from '@/shared/api'
import { connectAppError } from './app-error'

type ConnectRouter = Parameters<Parameters<typeof createRouterTransport>[0]>[0]

export interface FakeBillingOptions {
  calls?: string[]
  populated?: boolean
  registrationRequests?: Array<{ authKey: string; customerKey: string }>
  registerFailure?: 'EMAIL_VERIFICATION_REQUIRED' | 'CUSTOMER_KEY_MISMATCH' | 'BILLING_UNAVAILABLE'
  removeFailure?: 'SUBSCRIPTION_NEEDS_METHOD' | 'BILLING_UNAVAILABLE'
}

export function registerBillingService(router: ConnectRouter, options: FakeBillingOptions = {}) {
  const { calls, populated, registrationRequests, registerFailure, removeFailure } = options
  router.rpc(BillingService.method.getMyBilling, () => {
    calls?.push('GetMyBilling')
    return create(
      GetMyBillingResponseSchema,
      populated
        ? {
            customerKey: 'customer-key',
            subscription: { plan: ProtoPlan.PRO, term: ProtoTerm.MONTHLY, status: 'active' },
            paymentMethod: { cardLabel: '11 1234', registeredAt: '2026-09-08T00:00:00Z' },
            history: [{ id: 1n, kind: 'charge', krw: 7000n, createdAt: '2026-09-08T00:00:00Z' }],
            purchases: [
              { id: 'purchase-1', lotId: 'lot-1', credits: 100, usdCents: 100, krw: 1400n },
            ],
          }
        : { customerKey: 'customer-key' },
    )
  })
  router.rpc(BillingService.method.quotePrice, () =>
    create(QuotePriceResponseSchema, {
      usdCents: 500,
      krw: 7000n,
      krwPerUsdE4: 14000000n,
      rateDate: '2026-09-07',
    }),
  )
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
}
