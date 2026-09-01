import { cleanup, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

describe('session mutation failures', () => {
  it.each([
    {
      locale: 'ko' as const,
      loginId: '아이디',
      password: '비밀번호',
      submit: '로그인',
      message: '아이디 또는 비밀번호가 맞지 않아요.',
    },
    {
      locale: 'en' as const,
      loginId: 'Login ID',
      password: 'Password',
      submit: 'Log in',
      message: 'The login ID or password is incorrect.',
    },
  ])(
    'renders INVALID_CREDENTIALS without raw auth prose in $locale',
    async ({ locale, loginId, password, submit, message }) => {
      initializeI18n(locale)
      const user = userEvent.setup()
      const { router } = renderAppAt('/login', { loginFails: true })

      const loginIdField = await screen.findByLabelText(loginId)
      const passwordField = screen.getByLabelText(password)
      await user.type(loginIdField, 'ghost')
      await user.type(passwordField, 'pw')
      await user.click(screen.getByRole('button', { name: submit }))

      expect(await screen.findByRole('alert')).toHaveTextContent(message)
      expect(loginIdField).toHaveAttribute('aria-invalid', 'true')
      expect(passwordField).toHaveAccessibleDescription(message)
      expect(router.state.location.pathname).toBe('/login')
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[unauthenticated]')
    },
  )

  it.each([
    {
      locale: 'ko' as const,
      account: '내 계정',
      action: '로그아웃',
      message: '네트워크에 연결할 수 없어요.',
      sessionStillActive: '세션이 아직 살아 있으니 다시 시도해 주세요.',
    },
    {
      locale: 'en' as const,
      account: 'My account',
      action: 'Log out',
      message: 'Could not connect to the network.',
      sessionStillActive: 'Your session is still active, so please try again.',
    },
  ])(
    'renders NETWORK_UNAVAILABLE and preserves the live session in $locale',
    async ({ locale, account, action, message, sessionStillActive }) => {
      initializeI18n(locale)
      const user = userEvent.setup()
      const { router } = renderAppAt('/posts', {
        user: { id: 'alice' },
        logoutFails: true,
      })
      await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))

      await user.click(await screen.findByRole('button', { name: account }))
      await user.click(await screen.findByRole('button', { name: action }))

      const alert = await screen.findByRole('alert')
      expect(alert).toHaveTextContent(message)
      expect(alert).toHaveTextContent(sessionStillActive)
      expect(router.state.location.pathname).toBe('/posts')
      expect(document.body).not.toHaveTextContent('private backend prose')
      expect(document.body).not.toHaveTextContent('[unavailable]')
    },
  )
})
