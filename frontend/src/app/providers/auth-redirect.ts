import type { Transport } from '@connectrpc/connect'
import type { QueryClient } from '@tanstack/react-query'
import { onUnauthenticated } from '@/shared/api'
import { endSession } from '../model/end-session'
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
export function registerAuthRedirect({ router, queryClient }: AuthRedirectDeps): () => void {
  return onUnauthenticated(() => {
    // A visitor on a public page has no session to lose, and every public credential route
    // probes for one in beforeLoad — so that probe's own 401 reaches this listener. Acting
    // on it threw the visitor from /signup to /login before the form could render, and wiped
    // the caches of a page that was working. The event only means "a session died" on a
    // screen that required one.
    if (!onAuthenticatedScreen(router)) return

    queryClient.removeQueries()
    endSession()

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

/** Whether the screen the user is standing on is inside the authenticated subtree.
 *
 *  Read from the matched routes rather than from a list of public paths kept here: the route
 *  tree is where that fact lives, and a second list would drift the first time a public page
 *  is added. `/authenticated` is the layout every signed-in screen is a child of. */
function onAuthenticatedScreen(router: typeof AppRouter): boolean {
  return router.state.matches.some((match) => match.routeId === '/authenticated')
}
