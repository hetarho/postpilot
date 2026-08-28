import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { AuthService } from '@/shared/api'
import { getMeQueryKey } from './session-queries'

/** Revokes the session server-side and drops the local cache entry.
 *
 *  The cache is cleared only on success, on purpose. A failed Logout means the cookie is
 *  still valid, so clearing local state would be a lie the next page load exposes: the
 *  route guard would probe, find a live session, and bounce the user back into the app
 *  with no way to leave. Worse, the false "logged out" screen offers safety the still
 *  live HttpOnly cookie does not — and nothing in the browser can revoke it. */
export function useLogout() {
  const queryClient = useQueryClient()
  const transport = useTransport()

  return useMutation(AuthService.method.logout, {
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: getMeQueryKey(transport), exact: true })
    },
  })
}
