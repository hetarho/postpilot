import { beforeEach, describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import { GOOGLE_SIGN_IN_STORAGE_KEY } from '@/features/sign-in-with-google'
import { renderAppAt } from '@/test/app'

function storeAttempt(redirect = '/posts') {
  sessionStorage.setItem(
    GOOGLE_SIGN_IN_STORAGE_KEY,
    JSON.stringify({ state: 'expected-state', verifier: 'verifier', redirect }),
  )
}

describe('GoogleSignInCallbackPage', () => {
  beforeEach(() => sessionStorage.clear())

  it('refuses a state mismatch and clears the one-time attempt', async () => {
    storeAttempt()
    renderAppAt('/login/google/callback?code=code&state=wrong-state')

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Google 로그인을 완료하지 못했어요. 다시 시도해 주세요.',
    )
    expect(screen.getByRole('link', { name: '로그인으로' })).toHaveAttribute('href', '/login')
    expect(sessionStorage.getItem(GOOGLE_SIGN_IN_STORAGE_KEY)).toBeNull()
  })

  it.each([
    ['GOOGLE_EMAIL_UNVERIFIED', 'Google에서 인증된 이메일 주소를 확인할 수 없어요.'],
    ['GOOGLE_ACCOUNT_MISMATCH', '이 이메일은 다른 Google 계정에 이미 연결되어 있어요.'],
    ['GOOGLE_SIGNIN_DISABLED', '현재 Google 로그인을 사용할 수 없어요.'],
    ['TOO_MANY_ATTEMPTS', '요청이 너무 많아요. 2026. 10. 1. 오전 12:00 이후 다시 시도해 주세요.'],
  ] as const)('renders the localized %s refusal', async (reason, copy) => {
    storeAttempt()
    renderAppAt('/login/google/callback?code=code&state=expected-state', {
      ...(reason === 'TOO_MANY_ATTEMPTS'
        ? { tooManyAttempts: 'google' as const }
        : { googleFailure: reason }),
    })

    expect(await screen.findByRole('alert')).toHaveTextContent(copy)
    expect(sessionStorage.getItem(GOOGLE_SIGN_IN_STORAGE_KEY)).toBeNull()
  })

  it('seeds the session and refuses a stored off-site redirect', async () => {
    const calls: string[] = []
    storeAttempt('//evil.example.com')
    const { router } = renderAppAt('/login/google/callback?code=code&state=expected-state', {
      calls,
    })

    await waitFor(() => expect(router.state.location.pathname).toBe('/posts'))
    expect(calls.filter((call) => call === 'SignInWithGoogle')).toHaveLength(1)
    expect(calls.filter((call) => call === 'GetMe')).toHaveLength(0)
    expect(sessionStorage.getItem(GOOGLE_SIGN_IN_STORAGE_KEY)).toBeNull()
  })
})
