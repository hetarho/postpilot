import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { RegisterPaymentMethodButton } from '@/features/register-payment-method'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'

describe('RegisterPaymentMethodButton', () => {
  it('gates the hosted card action on a verified email', async () => {
    const transport = createFakeAuthTransport({
      user: { id: 'legacy', email: 'legacy@example.com', emailVerified: false },
    })
    render(<RegisterPaymentMethodButton customerKey="customer-key" clientKey="test-client" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })

    expect(await screen.findByRole('button', { name: '카드 등록' })).toBeDisabled()
    expect(screen.getByText('카드를 등록하려면 인증된 이메일이 필요합니다.')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '이메일 인증하기' })).toHaveAttribute(
      'href',
      '/account',
    )
  })

  it('is absent when the public client key is disabled', () => {
    const transport = createFakeAuthTransport({ user: { id: 'alice' } })
    const { container } = render(
      <RegisterPaymentMethodButton customerKey="customer-key" clientKey="" />,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )
    expect(container).toBeEmptyDOMElement()
  })
})
