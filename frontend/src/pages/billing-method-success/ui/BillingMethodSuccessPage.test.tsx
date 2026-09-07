import { screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ProtoPlan } from '@/shared/api'
import { renderAppAt } from '@/test/app'

describe('BillingMethodSuccessPage', () => {
  it('exchanges both redirect keys and replaces the callback with billing', async () => {
    const calls: string[] = []
    const registrationRequests: Array<{ authKey: string; customerKey: string }> = []
    const { router } = renderAppAt(
      '/billing/method/success?authKey=one-time-auth&customerKey=account-key',
      {
        user: {
          id: 'alice',
          plan: ProtoPlan.FREE,
          email: 'alice@example.com',
          emailVerified: true,
        },
        calls,
        billing: { registrationRequests },
      },
    )

    await waitFor(() => expect(router.state.location.pathname).toBe('/billing'))
    expect(registrationRequests).toEqual([{ authKey: 'one-time-auth', customerKey: 'account-key' }])
    expect(calls.filter((call) => call === 'RegisterPaymentMethod')).toHaveLength(1)
    expect(
      await screen.findByText('11 1234 카드가 등록되었고 100 크레딧 보너스가 지급되었습니다.'),
    ).toBeInTheDocument()
  })
})
