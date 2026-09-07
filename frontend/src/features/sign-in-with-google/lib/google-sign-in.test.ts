import { webcrypto } from 'node:crypto'
import { beforeEach, describe, expect, it } from 'vitest'
import {
  GOOGLE_SIGN_IN_STORAGE_KEY,
  type GoogleSignInAttempt,
  startGoogleSignIn,
} from './google-sign-in'

describe('startGoogleSignIn', () => {
  beforeEach(() => sessionStorage.clear())

  it('stores per-tab state and sends the verifier SHA-256 as a base64url challenge', async () => {
    let destination = ''
    await startGoogleSignIn('/posts/welcome', {
      clientID: 'client-id',
      origin: 'https://postpilot.example.com',
      crypto: webcrypto as unknown as Crypto,
      storage: sessionStorage,
      navigate: (url) => {
        destination = url
      },
    })

    const attempt = JSON.parse(
      sessionStorage.getItem(GOOGLE_SIGN_IN_STORAGE_KEY) ?? '',
    ) as GoogleSignInAttempt
    const digest = await webcrypto.subtle.digest(
      'SHA-256',
      new TextEncoder().encode(attempt.verifier),
    )
    const expectedChallenge = Buffer.from(digest).toString('base64url')
    const url = new URL(destination)

    expect(Buffer.from(attempt.state, 'base64url')).toHaveLength(16)
    expect(Buffer.from(attempt.verifier, 'base64url')).toHaveLength(32)
    expect(attempt.redirect).toBe('/posts/welcome')
    expect(url.origin + url.pathname).toBe('https://accounts.google.com/o/oauth2/v2/auth')
    expect(Object.fromEntries(url.searchParams)).toEqual({
      client_id: 'client-id',
      redirect_uri: 'https://postpilot.example.com/login/google/callback',
      response_type: 'code',
      scope: 'openid email',
      state: attempt.state,
      code_challenge: expectedChallenge,
      code_challenge_method: 'S256',
      prompt: 'select_account',
    })
  })
})
