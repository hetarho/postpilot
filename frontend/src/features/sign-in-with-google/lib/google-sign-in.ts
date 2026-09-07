import { GOOGLE_CLIENT_ID } from '@/shared/config'

export const GOOGLE_SIGN_IN_STORAGE_KEY = 'postpilot.google-signin'

export interface GoogleSignInAttempt {
  state: string
  verifier: string
  redirect: string
}

interface GoogleSignInEnvironment {
  clientID: string
  origin: string
  crypto: Crypto
  storage: Storage
  navigate: (url: string) => void
}

function browserEnvironment(): GoogleSignInEnvironment {
  return {
    clientID: GOOGLE_CLIENT_ID,
    origin: window.location.origin,
    crypto: window.crypto,
    storage: window.sessionStorage,
    navigate: (url) => window.location.assign(url),
  }
}

function base64url(bytes: Uint8Array): string {
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '')
}

function randomBase64url(crypto: Crypto, length: number): string {
  return base64url(crypto.getRandomValues(new Uint8Array(length)))
}

/** Starts the browser-owned half of authorization code + PKCE. */
export async function startGoogleSignIn(
  redirect?: string,
  environment: GoogleSignInEnvironment = browserEnvironment(),
): Promise<void> {
  if (!environment.clientID) throw new Error('Google sign-in is disabled')

  const state = randomBase64url(environment.crypto, 16)
  const verifier = randomBase64url(environment.crypto, 32)
  const digest = await environment.crypto.subtle.digest(
    'SHA-256',
    new TextEncoder().encode(verifier),
  )
  const challenge = base64url(new Uint8Array(digest))
  const attempt: GoogleSignInAttempt = { state, verifier, redirect: redirect ?? '' }
  try {
    environment.storage.setItem(GOOGLE_SIGN_IN_STORAGE_KEY, JSON.stringify(attempt))
  } catch (error) {
    throw new Error('Could not store Google sign-in state', { cause: error })
  }

  const callback = `${environment.origin}/login/google/callback`
  const query = new URLSearchParams({
    client_id: environment.clientID,
    redirect_uri: callback,
    response_type: 'code',
    scope: 'openid email',
    state,
    code_challenge: challenge,
    code_challenge_method: 'S256',
    prompt: 'select_account',
  })
  environment.navigate(`https://accounts.google.com/o/oauth2/v2/auth?${query}`)
}

export function readGoogleSignInAttempt(storage: Storage = window.sessionStorage) {
  try {
    const raw = storage.getItem(GOOGLE_SIGN_IN_STORAGE_KEY)
    if (!raw) return undefined
    const value = JSON.parse(raw) as Partial<GoogleSignInAttempt>
    if (
      typeof value.state !== 'string' ||
      typeof value.verifier !== 'string' ||
      typeof value.redirect !== 'string'
    ) {
      return undefined
    }
    return value as GoogleSignInAttempt
  } catch {
    return undefined
  }
}

export function clearGoogleSignInAttempt(storage: Storage = window.sessionStorage) {
  try {
    storage.removeItem(GOOGLE_SIGN_IN_STORAGE_KEY)
  } catch {
    // A blocked storage API is already equivalent to no resumable attempt.
  }
}
