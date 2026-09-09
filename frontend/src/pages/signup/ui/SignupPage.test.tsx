import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { AuthService } from '@/shared/api'
import { renderAppAt } from '@/test/app'

describe('SignupPage', () => {
  it('submits email and password, then shows the enumeration-safe mailed state', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/signup', { calls })

    expect(await screen.findByRole('link', { name: '로그인' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Postpilot이란?' })).toBeInTheDocument()
    const email = screen.getByLabelText('이메일')
    expect(email).toHaveAttribute('type', 'text')
    await user.type(email, 'Alice@Example.com')
    await user.type(screen.getByLabelText('비밀번호'), 'password1')
    await user.type(screen.getByLabelText('비밀번호 확인'), 'password1')
    await user.click(screen.getByRole('button', { name: '가입하기' }))

    expect(await screen.findByRole('heading', { name: '메일을 확인해 주세요' })).toBeInTheDocument()
    expect(screen.getByText(/Alice@Example.com/)).toBeInTheDocument()
    expect(calls.filter((call) => call === 'Signup')).toHaveLength(1)

    await user.click(screen.getByRole('button', { name: '다시 보내기' }))
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ResendVerification')).toHaveLength(1),
    )
  })

  // AUTH-42: the screen names itself and leads back to login through a sentence whose link keeps
  // the bare verb as its name.
  it('is titled 회원가입 and leads to login through a question with its answer', async () => {
    renderAppAt('/signup?redirect=%2Fposts')

    expect(await screen.findByRole('heading', { level: 1, name: '회원가입' })).toBeInTheDocument()
    expect(screen.getByText('이미 계정이 있으세요?')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '로그인' })).toHaveAttribute(
      'href',
      '/login?redirect=%2Fposts',
    )
    expect(screen.getByLabelText('비밀번호 확인')).toHaveAttribute('autocomplete', 'new-password')
  })

  it('refuses a password mismatch in the form and sends nothing', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/signup', { calls })

    await user.type(await screen.findByLabelText('이메일'), 'alice@example.com')
    const password = screen.getByLabelText('비밀번호')
    const confirm = screen.getByLabelText('비밀번호 확인')
    await user.type(password, 'password1')
    await user.type(confirm, 'password2')
    await user.click(screen.getByRole('button', { name: '가입하기' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('비밀번호가 서로 달라요')
    expect(password).toHaveAttribute('aria-invalid', 'true')
    expect(confirm).toHaveAttribute('aria-invalid', 'true')
    expect(calls.filter((call) => call === 'Signup')).toHaveLength(0)

    // Retyping clears the mark; a matching pair then submits as before.
    await user.clear(confirm)
    expect(password).not.toHaveAttribute('aria-invalid')
    await user.type(confirm, 'password1')
    await user.click(screen.getByRole('button', { name: '가입하기' }))
    expect(await screen.findByRole('heading', { name: '메일을 확인해 주세요' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { level: 1, name: '회원가입' })).toBeInTheDocument()
    expect(calls.filter((call) => call === 'Signup')).toHaveLength(1)
  })

  it('reverse-guards an already signed-in visitor', async () => {
    const { router } = renderAppAt('/signup', { user: { id: 'alice' } })
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
  })

  it('shows a localized retry instant when signup is throttled', async () => {
    const user = userEvent.setup()
    renderAppAt('/signup', { tooManyAttempts: 'signup' })

    await user.type(await screen.findByLabelText('이메일'), 'alice@example.com')
    await user.type(screen.getByLabelText('비밀번호'), 'password1')
    await user.type(screen.getByLabelText('비밀번호 확인'), 'password1')
    await user.click(screen.getByRole('button', { name: '가입하기' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      '요청이 너무 많아요. 2026. 10. 1. 오전 12:00 이후 다시 시도해 주세요.',
    )
  })

  it('still renders when the reverse-guard session check is unavailable', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(AuthService.method.getMe, () => {
        throw new ConnectError('unavailable', Code.Unavailable)
      })
    })

    renderAppAt('/signup', { transport })

    expect(await screen.findByRole('button', { name: '가입하기' })).toBeInTheDocument()
  })
})
