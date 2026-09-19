// The session write itself is the session entity's CRUD (ARCH-14); this slice owns the OAuth
// attempt — the state, the verifier and where to land — and re-exports the hook so the
// callback screen reads one verb from one place.
export { useSignInWithGoogle } from '@/entities/session'
export { GoogleSignInButton } from './ui/GoogleSignInButton'
export {
  GOOGLE_SIGN_IN_STORAGE_KEY,
  clearGoogleSignInAttempt,
  readGoogleSignInAttempt,
  startGoogleSignIn,
} from './lib/google-sign-in'
export type { GoogleSignInAttempt } from './lib/google-sign-in'
