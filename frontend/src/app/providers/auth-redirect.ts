import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { getMeQueryKey } from '@/entities/session'
import { onUnauthenticated } from '@/shared/api'
import type { router as AppRouter } from '../routes/router'

interface AuthRedirectDeps {
  router: typeof AppRouter
  queryClient: QueryClient
  transport: Transport
}

/** Sends the user back to /login when a session dies mid-use.
 *
 *  The route guard only runs on navigation, so a session that expires while the user
 *  sits on a screen would otherwise surface as an unexplained failed request. This
 *  closes that gap from anywhere in the app. Returns the unsubscribe function.
 *
 *  Import direction matters: this reads the router, so the router must never import
 *  this — App.tsx is what wires the two together. */
export function registerAuthRedirect({
  router,
  queryClient,
  transport,
}: AuthRedirectDeps): () => void {
  return onUnauthenticated(() => {
    queryClient.removeQueries({ queryKey: getMeQueryKey(transport), exact: true })

    // Already there: navigating again would re-run the login route's beforeLoad, whose
    // own session probe can emit this very event.
    if (router.state.location.pathname === '/login') return

    void router.navigate({
      to: '/login',
      search: { redirect: router.state.location.href },
      replace: true,
    })
  })
}
