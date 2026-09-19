import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { appFailureFromConnect, AuthService } from '@/shared/api'
import { seedSessionCache } from './session-queries'

/** Exchanges the Google authorization code for a session and seeds the session cache from the
 *  answer — the same reason `useLogin` seeds it: the route guard reads that entry, and the
 *  navigation that follows would otherwise find no session and bounce back to /login. The OAuth
 *  attempt itself (state, verifier, redirect) belongs to `features/sign-in-with-google`. */
export function useSignInWithGoogle() {
  const queryClient = useQueryClient()
  const transport = useTransport()
  const mutation = useMutation(AuthService.method.signInWithGoogle, {
    onSuccess: (data) => seedSessionCache(queryClient, transport, data),
  })
  return {
    ...mutation,
    failure: mutation.error ? appFailureFromConnect(mutation.error) : undefined,
  }
}
