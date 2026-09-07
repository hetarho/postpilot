import { loadScript } from '@/shared/lib'

export const TOSS_PAYMENTS_SDK = 'https://js.tosspayments.com/v2/standard'

interface TossPayment {
  requestBillingAuth(options: {
    method: 'CARD'
    successUrl: string
    failUrl: string
    customerEmail: string
  }): Promise<void>
}

interface TossPaymentsSDK {
  payment(options: { customerKey: string }): TossPayment
}

type TossPaymentsFactory = (clientKey: string) => TossPaymentsSDK

interface TossEnvironment {
  load: (src: string) => Promise<void>
  origin: string
  factory: () => TossPaymentsFactory | undefined
}

function browserEnvironment(): TossEnvironment {
  return {
    load: loadScript,
    origin: window.location.origin,
    factory: () => (window as Window & { TossPayments?: TossPaymentsFactory }).TossPayments,
  }
}

export async function openTossBillingAuth(
  input: { clientKey: string; customerKey: string; customerEmail: string },
  environment: TossEnvironment = browserEnvironment(),
) {
  await environment.load(TOSS_PAYMENTS_SDK)
  const factory = environment.factory()
  if (!factory) throw new Error('Toss Payments SDK is unavailable')
  await factory(input.clientKey)
    .payment({ customerKey: input.customerKey })
    .requestBillingAuth({
      method: 'CARD',
      successUrl: `${environment.origin}/billing/method/success`,
      failUrl: `${environment.origin}/billing/method/fail`,
      customerEmail: input.customerEmail,
    })
}
