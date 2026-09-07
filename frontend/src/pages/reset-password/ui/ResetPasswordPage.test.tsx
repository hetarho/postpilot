import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

describe('ResetPasswordPage', () => {
  it('submits the token and offers login after success', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/reset-password?token=raw-token', { calls })

    await user.type(await screen.findByLabelText('새 비밀번호'), 'new-password')
    await user.click(screen.getByRole('button', { name: '비밀번호 재설정' }))

    expect(await screen.findByRole('heading', { name: '비밀번호를 바꿨어요' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '로그인' })).toHaveAttribute('href', '/login')
    expect(calls.filter((call) => call === 'ResetPassword')).toHaveLength(1)
  })

  it('localizes an invalid link and offers a fresh reset request', async () => {
    const user = userEvent.setup()
    renderAppAt('/reset-password?token=expired', { resetPasswordFails: true })

    await user.type(await screen.findByLabelText('새 비밀번호'), 'new-password')
    await user.click(screen.getByRole('button', { name: '비밀번호 재설정' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      '비밀번호 재설정 링크가 올바르지 않거나 만료됐어요.',
    )
    expect(screen.getByRole('link', { name: '재설정 메일 다시 받기' })).toHaveAttribute(
      'href',
      '/forgot-password',
    )
  })

  it('shows a localized retry instant when reset attempts are throttled', async () => {
    const user = userEvent.setup()
    renderAppAt('/reset-password?token=raw-token', { tooManyAttempts: 'reset' })

    await user.type(await screen.findByLabelText('새 비밀번호'), 'new-password')
    await user.click(screen.getByRole('button', { name: '비밀번호 재설정' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      '요청이 너무 많아요. 2026. 10. 1. 오전 12:00 이후 다시 시도해 주세요.',
    )
  })
})
