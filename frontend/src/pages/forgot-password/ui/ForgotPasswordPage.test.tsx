import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { AuthService } from '@/shared/api'
import { renderAppAt } from '@/test/app'

describe('ForgotPasswordPage', () => {
  it('submits an address and shows the enumeration-safe mailed state', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    renderAppAt('/forgot-password', { calls })

    await user.type(await screen.findByLabelText('이메일'), 'alice@example.com')
    await user.click(screen.getByRole('button', { name: '재설정 메일 보내기' }))

    expect(await screen.findByRole('heading', { name: '메일을 확인해 주세요' })).toBeInTheDocument()
    expect(screen.getByText(/alice@example.com/)).toBeInTheDocument()
    expect(calls.filter((call) => call === 'RequestPasswordReset')).toHaveLength(1)
  })

  it('reverse-guards a signed-in visitor', async () => {
    const { router } = renderAppAt('/forgot-password', { user: { id: 'alice' } })
    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
  })

  it('still renders when the reverse-guard session check is unavailable', async () => {
    const transport = createRouterTransport(({ rpc }) => {
      rpc(AuthService.method.getMe, () => {
        throw new ConnectError('unavailable', Code.Unavailable)
      })
    })
    renderAppAt('/forgot-password', { transport })

    expect(await screen.findByRole('button', { name: '재설정 메일 보내기' })).toBeInTheDocument()
  })
})
