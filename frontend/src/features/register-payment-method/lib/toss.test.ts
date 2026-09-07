import { describe, expect, it, vi } from 'vitest'
import { openTossBillingAuth, TOSS_PAYMENTS_SDK } from './toss'

describe('openTossBillingAuth', () => {
  it('opens the hosted card registration with absolute callbacks and the account identity', async () => {
    const requestBillingAuth = vi.fn().mockResolvedValue(undefined)
    const payment = vi.fn(() => ({ requestBillingAuth }))
    const factory = vi.fn(() => ({ payment }))
    const load = vi.fn().mockResolvedValue(undefined)

    await openTossBillingAuth(
      { clientKey: 'client-key', customerKey: 'customer-key', customerEmail: 'alice@example.com' },
      { load, origin: 'https://postpilot.test', factory: () => factory },
    )

    expect(load).toHaveBeenCalledWith(TOSS_PAYMENTS_SDK)
    expect(factory).toHaveBeenCalledWith('client-key')
    expect(payment).toHaveBeenCalledWith({ customerKey: 'customer-key' })
    expect(requestBillingAuth).toHaveBeenCalledWith({
      method: 'CARD',
      successUrl: 'https://postpilot.test/billing/method/success',
      failUrl: 'https://postpilot.test/billing/method/fail',
      customerEmail: 'alice@example.com',
    })
  })

  it('carries a safe in-app return destination through both provider callbacks', async () => {
    const requestBillingAuth = vi.fn().mockResolvedValue(undefined)
    const factory = vi.fn(() => ({ payment: () => ({ requestBillingAuth }) }))

    await openTossBillingAuth(
      {
        clientKey: 'client-key',
        customerKey: 'customer-key',
        customerEmail: 'alice@example.com',
        returnTo: '/billing/checkout?tier=pro',
      },
      {
        load: vi.fn().mockResolvedValue(undefined),
        origin: 'https://postpilot.test',
        factory: () => factory,
      },
    )

    expect(requestBillingAuth).toHaveBeenCalledWith(
      expect.objectContaining({
        successUrl:
          'https://postpilot.test/billing/method/success?redirect=%2Fbilling%2Fcheckout%3Ftier%3Dpro',
        failUrl:
          'https://postpilot.test/billing/method/fail?redirect=%2Fbilling%2Fcheckout%3Ftier%3Dpro',
      }),
    )
  })
})
