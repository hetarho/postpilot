import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

describe('AccountPage', () => {
  it('shows identity and lets an emailless account request verification', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/account', { calls, user: { id: 'legacy' } })

    expect(await screen.findByRole('heading', { name: '계정 설정' })).toBeInTheDocument()
    expect(screen.getByText('legacy')).toBeInTheDocument()
    expect(screen.getByText('등록된 이메일 없음')).toBeInTheDocument()
    await user.type(screen.getByLabelText('이메일'), 'legacy@example.com')
    await user.click(screen.getByRole('button', { name: '인증 메일 보내기' }))
    expect(await screen.findByRole('status')).toHaveTextContent('legacy@example.com')
    expect(calls.filter((call) => call === 'RegisterEmail')).toHaveLength(1)
  })

  it('shows a verified email without the registration form', async () => {
    renderAppAt('/account', {
      user: { id: 'alice', email: 'alice@example.com', emailVerified: true },
    })

    expect(await screen.findByText('alice@example.com')).toBeInTheDocument()
    expect(screen.getByText('인증됨')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '인증 메일 보내기' })).not.toBeInTheDocument()
  })
})
