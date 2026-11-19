import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

describe('AccountPage', () => {
  it('shows the account as rows and opens email registration from the email pencil', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/account', { calls, user: { id: 'legacy' } })

    expect(await screen.findByRole('heading', { name: '계정 설정' })).toBeInTheDocument()
    expect(screen.getByText('아이디')).toBeInTheDocument()
    expect(screen.getByText('legacy')).toBeInTheDocument()
    expect(screen.getByText('등록된 이메일 없음')).toBeInTheDocument()
    // The form waits behind the row's pencil.
    expect(screen.queryByRole('button', { name: '인증 메일 보내기' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: '이메일 등록' }))
    await user.type(screen.getByLabelText('이메일'), 'legacy@example.com')
    await user.click(screen.getByRole('button', { name: '인증 메일 보내기' }))
    expect(await screen.findByRole('status')).toHaveTextContent('legacy@example.com')
    expect(calls.filter((call) => call === 'RegisterEmail')).toHaveLength(1)
  })

  it('closes an opened form back to its row on cancel', async () => {
    const user = userEvent.setup()
    renderAppAt('/account', {
      user: { id: 'bob', email: 'bob@example.com', emailVerified: false },
    })

    expect(await screen.findByText('미인증')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '이메일 등록' }))
    expect(screen.getByLabelText('이메일')).toHaveValue('bob@example.com')

    await user.click(screen.getByRole('button', { name: '취소' }))
    expect(screen.queryByLabelText('이메일')).not.toBeInTheDocument()
    expect(screen.getByText('bob@example.com')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '이메일 등록' })).toBeInTheDocument()
  })

  it('shows a verified email with no pencil', async () => {
    renderAppAt('/account', {
      user: { id: 'alice', email: 'alice@example.com', emailVerified: true },
    })

    expect(await screen.findByText('alice@example.com')).toBeInTheDocument()
    expect(screen.getByText('인증됨')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '이메일 등록' })).not.toBeInTheDocument()
  })

  it('keeps a wrong current password inline', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/account', {
      user: { id: 'alice', hasPassword: true },
      changePasswordFails: 'wrong-current',
    })

    expect(screen.queryByLabelText('현재 비밀번호')).not.toBeInTheDocument()
    await user.click(await screen.findByRole('button', { name: '비밀번호 변경' }))
    await user.type(screen.getByLabelText('현재 비밀번호'), 'wrong')
    await user.type(screen.getByLabelText('새 비밀번호'), 'new-password')
    await user.click(screen.getByRole('button', { name: '비밀번호 변경' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('현재 비밀번호가 맞지 않아요.')
    expect(router.state.location.pathname).toBe('/account')
  })

  it('drops the session and lands on login with a one-time success notice', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const { router } = renderAppAt('/account', {
      calls,
      user: { id: 'alice', hasPassword: true },
    })

    await user.click(await screen.findByRole('button', { name: '비밀번호 변경' }))
    await user.type(screen.getByLabelText('현재 비밀번호'), 's3cret')
    await user.type(screen.getByLabelText('새 비밀번호'), 'new-password')
    await user.click(screen.getByRole('button', { name: '비밀번호 변경' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/login'))
    expect(await screen.findByRole('status')).toHaveTextContent(
      '비밀번호를 바꿨어요. 다시 로그인해 주세요.',
    )
    await waitFor(() => expect(router.state.location.state.notice).toBeUndefined())
    expect(calls.filter((call) => call === 'ChangePassword')).toHaveLength(1)
  })

  it('keeps the no-password line with no pencil and no link', async () => {
    renderAppAt('/account', {
      user: {
        id: 'google-user',
        email: 'google@example.com',
        emailVerified: true,
        hasPassword: false,
      },
    })

    expect(await screen.findByText(/로그아웃한 뒤 비밀번호 찾기/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '비밀번호 변경' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /비밀번호 찾기/ })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('현재 비밀번호')).not.toBeInTheDocument()
  })
})
