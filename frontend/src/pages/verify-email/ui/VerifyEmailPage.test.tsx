import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

describe('VerifyEmailPage', () => {
  it('consumes the token once and offers login after success', async () => {
    const calls: string[] = []
    renderAppAt('/verify-email?token=raw-token', { calls })

    expect(await screen.findByRole('heading', { name: '이메일 인증 완료' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '로그인' })).toHaveAttribute('href', '/login')
    expect(calls.filter((call) => call === 'VerifyEmail')).toHaveLength(1)
  })

  it('localizes an invalid link and resends from the recovery form', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/verify-email?token=expired', { calls, verificationFails: true })

    expect(await screen.findByRole('alert')).toHaveTextContent(
      '인증 링크가 올바르지 않거나 만료됐어요.',
    )
    await user.type(screen.getByLabelText('이메일'), 'alice@example.com')
    await user.click(screen.getByRole('button', { name: '다시 보내기' }))
    expect(await screen.findByRole('status')).toHaveTextContent('alice@example.com')
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ResendVerification')).toHaveLength(1),
    )
  })
})
