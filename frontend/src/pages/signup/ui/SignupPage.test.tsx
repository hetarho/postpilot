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
    await user.click(screen.getByRole('button', { name: '가입하기' }))

    expect(await screen.findByRole('heading', { name: '메일을 확인해 주세요' })).toBeInTheDocument()
    expect(screen.getByText(/Alice@Example.com/)).toBeInTheDocument()
    expect(calls.filter((call) => call === 'Signup')).toHaveLength(1)

    await user.click(screen.getByRole('button', { name: '다시 보내기' }))
    await waitFor(() =>
      expect(calls.filter((call) => call === 'ResendVerification')).toHaveLength(1),
    )
  })

  it('reverse-guards an already signed-in visitor', async () => {
    const { router } = renderAppAt('/signup', { user: { id: 'alice' } })
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
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
